package workers

import (
	"context"
	"encoding/json"

	"github.com/Beka01247/bitpanda-aml/internal/application"
	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

const QueueReportJobs = "q_report_jobs"

type ReportWorker struct {
	generateReportUseCase    *application.GenerateReportUseCase
	handleCheckFailedUseCase *application.HandleCheckFailedUseCase
	messageBus               domain.MessageBus
	logger                   *zap.SugaredLogger
	ctx                      context.Context
	cancel                   context.CancelFunc
}

func NewReportWorker(
	generateReportUseCase *application.GenerateReportUseCase,
	handleCheckFailedUseCase *application.HandleCheckFailedUseCase,
	messageBus domain.MessageBus,
	logger *zap.SugaredLogger,
) *ReportWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReportWorker{
		generateReportUseCase:    generateReportUseCase,
		handleCheckFailedUseCase: handleCheckFailedUseCase,
		messageBus:               messageBus,
		logger:                   logger,
		ctx:                      ctx,
		cancel:                   cancel,
	}
}

func (w *ReportWorker) Start() error {
	w.logger.Info("starting report worker")

	routingKeys := []string{domain.EventAMLCheckCompleted, domain.EventAMLCheckFailed}

	return w.messageBus.Subscribe(w.ctx, QueueReportJobs, routingKeys, w.handleMessage)
}

func (w *ReportWorker) Stop() {
	w.logger.Info("stopping report worker")
	w.cancel()
}

func (w *ReportWorker) handleMessage(body []byte) error {
	var event domain.Event
	if err := json.Unmarshal(body, &event); err != nil {
		w.logger.Errorw("failed to unmarshal event", "error", err)
		return err
	}

	w.logger.Infow("processing event", "event_type", event.Type, "event_id", event.ID)

	switch event.Type {
	case domain.EventAMLCheckCompleted:
		return w.handleAMLCheckCompleted(&event)
	case domain.EventAMLCheckFailed:
		return w.handleAMLCheckFailed(&event)
	default:
		w.logger.Warnw("unknown event type", "event_type", event.Type)
		return nil
	}
}

func (w *ReportWorker) handleAMLCheckCompleted(event *domain.Event) error {
	// parse payload
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		w.logger.Errorw("failed to marshal payload", "error", err)
		return err
	}

	var payload domain.AMLCheckCompletedPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		w.logger.Errorw("failed to unmarshal payload", "error", err)
		return err
	}

	// generate report
	ctx := context.Background()
	return w.generateReportUseCase.Execute(ctx, payload.CheckID, payload.RiskScore, payload.RiskLevel, payload.Categories, payload.Sanctions)
}

func (w *ReportWorker) handleAMLCheckFailed(event *domain.Event) error {
	// parse payload
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		w.logger.Errorw("failed to marshal payload", "error", err)
		return err
	}

	var payload domain.AMLCheckFailedPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		w.logger.Errorw("failed to unmarshal payload", "error", err)
		return err
	}

	ctx := context.Background()
	return w.handleCheckFailedUseCase.Execute(ctx, payload.CheckID, payload.ErrorMessage)
}
