package workers

import (
	"context"
	"encoding/json"

	"github.com/Beka01247/bitpanda-aml/internal/application"
	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

const QueueAMLRequests = "q_aml_requests"

type AMLWorker struct {
	processUseCase *application.ProcessAMLCheckUseCase
	messageBus     domain.MessageBus
	logger         *zap.SugaredLogger
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewAMLWorker(
	processUseCase *application.ProcessAMLCheckUseCase,
	messageBus domain.MessageBus,
	logger *zap.SugaredLogger,
) *AMLWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &AMLWorker{
		processUseCase: processUseCase,
		messageBus:     messageBus,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (w *AMLWorker) Start() error {
	w.logger.Info("starting aml worker")

	routingKeys := []string{domain.EventAMLCheckRequested}

	return w.messageBus.Subscribe(w.ctx, QueueAMLRequests, routingKeys, w.handleMessage)
}

func (w *AMLWorker) Stop() {
	w.logger.Info("stopping aml worker")
	w.cancel()
}

func (w *AMLWorker) handleMessage(body []byte) error {
	var event domain.Event
	if err := json.Unmarshal(body, &event); err != nil {
		w.logger.Errorw("failed to unmarshal event", "error", err)
		return err
	}

	w.logger.Infow("processing event", "event_type", event.Type, "event_id", event.ID)

	switch event.Type {
	case domain.EventAMLCheckRequested:
		return w.handleAMLCheckRequested(&event)
	default:
		w.logger.Warnw("unknown event type", "event_type", event.Type)
		return nil
	}
}

func (w *AMLWorker) handleAMLCheckRequested(event *domain.Event) error {
	// parse payload
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		w.logger.Errorw("failed to marshal payload", "error", err)
		return err
	}

	var payload domain.AMLCheckRequestedPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		w.logger.Errorw("failed to unmarshal payload", "error", err)
		return err
	}

	// process check
	ctx := context.Background()
	return w.processUseCase.Execute(ctx, payload.CheckID, payload.Address, payload.Currency)
}
