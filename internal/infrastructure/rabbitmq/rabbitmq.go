package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	ExchangeName    = "aml.events"
	ExchangeType    = "topic"
	DLQExchangeName = "aml.dlq"
	MaxRetries      = 3
)

type RabbitMQBus struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	logger  *zap.SugaredLogger
}

func NewRabbitMQBus(url string, logger *zap.SugaredLogger) (*RabbitMQBus, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	err = channel.ExchangeDeclare(
		ExchangeName,
		ExchangeType,
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// declare dlq exchange
	err = channel.ExchangeDeclare(
		DLQExchangeName,
		"fanout",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare dlq exchange: %w", err)
	}

	logger.Infow("rabbitmq connected", "exchange", ExchangeName)

	return &RabbitMQBus{
		conn:    conn,
		channel: channel,
		logger:  logger,
	}, nil
}

func (b *RabbitMQBus) Publish(ctx context.Context, routingKey string, event *domain.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = b.channel.PublishWithContext(
		ctx,
		ExchangeName,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	b.logger.Debugw("event published", "routing_key", routingKey, "event_type", event.Type)

	return nil
}

// subscribe subscribes to events from the message bus
func (b *RabbitMQBus) Subscribe(ctx context.Context, queueName string, routingKeys []string, handler func([]byte) error) error {
	// declare dlq queue
	dlqQueueName := queueName + ".dlq"
	dlqQueue, err := b.channel.QueueDeclare(
		dlqQueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dlq queue: %w", err)
	}

	// bind dlq queue to dlq exchange
	err = b.channel.QueueBind(
		dlqQueue.Name,
		"", // routing key (fanout ignores this)
		DLQExchangeName,
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind dlq queue: %w", err)
	}

	// declare main queue with dlq configuration
	queue, err := b.channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// bind queue to routing keys
	for _, routingKey := range routingKeys {
		err = b.channel.QueueBind(
			queue.Name,
			routingKey,
			ExchangeName,
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue to routing key %s: %w", routingKey, err)
		}
	}

	// set QoS
	err = b.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set qos: %w", err)
	}

	// Consume messages
	msgs, err := b.channel.Consume(
		queue.Name,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	b.logger.Infow("subscribed to queue", "queue", queueName, "routing_keys", routingKeys)

	go func() {
		for {
			select {
			case <-ctx.Done():
				b.logger.Infow("consumer stopped", "queue", queueName)
				return
			case msg, ok := <-msgs:
				if !ok {
					b.logger.Warnw("message channel closed", "queue", queueName)
					return
				}

				b.logger.Debugw("message received", "queue", queueName, "routing_key", msg.RoutingKey)

				err := handler(msg.Body)
				if err != nil {
					// get retry count from headers
					retryCount := int32(0)
					if msg.Headers != nil {
						if count, ok := msg.Headers["x-retry-count"].(int32); ok {
							retryCount = count
						}
					}

					b.logger.Errorw("failed to handle message", "queue", queueName, "error", err, "retry_count", retryCount)

					if retryCount >= MaxRetries {
						// max retries exceeded, send to dlq
						b.logger.Errorw("max retries exceeded, sending to dlq", "queue", queueName, "retry_count", retryCount)
						err := b.channel.PublishWithContext(
							ctx,
							DLQExchangeName,
							"", // routing key
							false,
							false,
							amqp.Publishing{
								ContentType:  msg.ContentType,
								Body:         msg.Body,
								DeliveryMode: amqp.Persistent,
								Headers: amqp.Table{
									"x-original-queue":   queueName,
									"x-original-routing": msg.RoutingKey,
									"x-retry-count":      retryCount,
									"x-last-error":       err.Error(),
									"x-failed-timestamp": msg.Timestamp,
								},
							},
						)
						if err != nil {
							b.logger.Errorw("failed to publish to dlq", "error", err)
							msg.Nack(false, false) // don't requeue
						} else {
							msg.Ack(false) // acknowledge original message
						}
					} else {
						// increment retry count and requeue
						retryCount++
						err := b.channel.PublishWithContext(
							ctx,
							ExchangeName,
							msg.RoutingKey,
							false,
							false,
							amqp.Publishing{
								ContentType:  msg.ContentType,
								Body:         msg.Body,
								DeliveryMode: amqp.Persistent,
								Headers: amqp.Table{
									"x-retry-count": retryCount,
								},
							},
						)
						if err != nil {
							b.logger.Errorw("failed to republish message", "error", err)
							msg.Nack(false, true) // fallback to basic requeue
						} else {
							msg.Ack(false) // acknowledge original message
						}
					}
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

func (b *RabbitMQBus) Close() error {
	if b.channel != nil {
		if err := b.channel.Close(); err != nil {
			b.logger.Errorw("failed to close channel", "error", err)
		}
	}
	if b.conn != nil {
		if err := b.conn.Close(); err != nil {
			b.logger.Errorw("failed to close connection", "error", err)
		}
	}
	b.logger.Info("rabbitmq connection closed")
	return nil
}
