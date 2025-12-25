package application

import (
	"context"
	"fmt"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type GenerateReportUseCase struct {
	repository    domain.AMLCheckRepository
	reportStorage domain.ReportStorage
	messageBus    domain.MessageBus
	billingHook   domain.BillingHook
	reportTTL     time.Duration
	logger        *zap.SugaredLogger
}

func NewGenerateReportUseCase(
	repository domain.AMLCheckRepository,
	reportStorage domain.ReportStorage,
	messageBus domain.MessageBus,
	billingHook domain.BillingHook,
	reportTTL time.Duration,
	logger *zap.SugaredLogger,
) *GenerateReportUseCase {
	return &GenerateReportUseCase{
		repository:    repository,
		reportStorage: reportStorage,
		messageBus:    messageBus,
		billingHook:   billingHook,
		reportTTL:     reportTTL,
		logger:        logger,
	}
}

// executes the generate report use case
func (u *GenerateReportUseCase) Execute(ctx context.Context, checkID string, riskScore int, riskLevel domain.RiskLevel, categories []string, sanctions *domain.SanctionsResult) error {
	u.logger.Infow("generating report", "check_id", checkID)

	// get check
	check, err := u.repository.Get(ctx, checkID)
	if err != nil {
		u.logger.Errorw("failed to get check", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to get check: %w", err)
	}

	if check == nil {
		return fmt.Errorf("check not found")
	}

	// generate PDF
	pdfData, err := GeneratePDF(check.Address, check.Currency, riskScore, riskLevel, categories, sanctions, checkID)
	if err != nil {
		u.logger.Errorw("failed to generate pdf", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to generate pdf: %w", err)
	}

	// validate PDF
	if len(pdfData) < 1024 || string(pdfData[:4]) != "%PDF" {
		u.logger.Errorw("invalid pdf generated", "check_id", checkID, "size", len(pdfData))
		return fmt.Errorf("invalid pdf generated")
	}

	// Store PDF
	reportKey := fmt.Sprintf("%s.pdf", checkID)
	if err := u.reportStorage.Put(ctx, reportKey, pdfData, u.reportTTL); err != nil {
		u.logger.Errorw("failed to store report", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to store report: %w", err)
	}

	// update check
	check.MarkCompleted(riskScore, riskLevel, categories, sanctions, reportKey)
	if err := u.repository.Update(ctx, check); err != nil {
		u.logger.Errorw("failed to update check", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to update check: %w", err)
	}

	// publish report ready event
	event := domain.NewEvent(domain.EventAMLReportReady, &domain.AMLReportReadyPayload{
		CheckID:   checkID,
		ReportKey: reportKey,
	})

	if err := u.messageBus.Publish(ctx, domain.EventAMLReportReady, event); err != nil {
		u.logger.Errorw("failed to publish report ready event", "check_id", checkID, "error", err)
	}

	// billing hook (non-blocking)
	if err := u.billingHook.OnCheckCompleted(ctx, check); err != nil {
		u.logger.Warnw("billing hook failed", "check_id", checkID, "error", err)
	}

	u.logger.Infow("report generated", "check_id", checkID, "report_key", reportKey, "pdf_size", len(pdfData))

	return nil
}
