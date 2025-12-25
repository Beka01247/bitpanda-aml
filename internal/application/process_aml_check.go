package application

import (
	"context"
	"fmt"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type ProcessAMLCheckUseCase struct {
	amlProvider       domain.AMLProvider
	sanctionsProvider domain.SanctionsProvider
	repository        domain.AMLCheckRepository
	messageBus        domain.MessageBus
	logger            *zap.SugaredLogger
}

func NewProcessAMLCheckUseCase(
	amlProvider domain.AMLProvider,
	sanctionsProvider domain.SanctionsProvider,
	repository domain.AMLCheckRepository,
	messageBus domain.MessageBus,
	logger *zap.SugaredLogger,
) *ProcessAMLCheckUseCase {
	return &ProcessAMLCheckUseCase{
		amlProvider:       amlProvider,
		sanctionsProvider: sanctionsProvider,
		repository:        repository,
		messageBus:        messageBus,
		logger:            logger,
	}
}

// executes the process AML check use case
func (u *ProcessAMLCheckUseCase) Execute(ctx context.Context, checkID, address, currency string) error {
	u.logger.Infow("processing aml check", "check_id", checkID, "provider", u.amlProvider.Name())

	startTime := time.Now()

	// call AML provider
	amlResult, err := u.amlProvider.CheckAddress(ctx, address, currency)
	if err != nil {
		u.logger.Errorw("aml provider failed", "check_id", checkID, "provider", u.amlProvider.Name(), "error", err)
		return u.publishFailedEvent(ctx, checkID, fmt.Sprintf("AML check failed: %v", err))
	}

	u.logger.Infow("aml provider completed",
		"check_id", checkID,
		"provider", u.amlProvider.Name(),
		"latency_ms", time.Since(startTime).Milliseconds(),
		"risk_score", amlResult.RiskScore)

	// call Chainalysis sanctions provider
	sanctionsStart := time.Now()
	sanctionsResult, err := u.sanctionsProvider.CheckAddress(ctx, address)
	if err != nil {
		// sanctions failure should not break the pipeline
		u.logger.Warnw("sanctions provider failed", "check_id", checkID, "provider", u.sanctionsProvider.Name(), "error", err)
		sanctionsResult = &domain.SanctionsResult{
			Hit:             false,
			Identifications: []domain.SanctionsIdentification{},
		}
	} else {
		u.logger.Infow("sanctions provider completed",
			"check_id", checkID,
			"provider", u.sanctionsProvider.Name(),
			"latency_ms", time.Since(sanctionsStart).Milliseconds(),
			"hit", sanctionsResult.Hit)
	}

	// publish completed event
	event := domain.NewEvent(domain.EventAMLCheckCompleted, &domain.AMLCheckCompletedPayload{
		CheckID:    checkID,
		RiskScore:  amlResult.RiskScore,
		RiskLevel:  amlResult.RiskLevel,
		Categories: amlResult.Categories,
		Sanctions:  sanctionsResult,
	})

	if err := u.messageBus.Publish(ctx, domain.EventAMLCheckCompleted, event); err != nil {
		u.logger.Errorw("failed to publish completed event", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to publish completed event: %w", err)
	}

	return nil
}

func (u *ProcessAMLCheckUseCase) publishFailedEvent(ctx context.Context, checkID, errorMessage string) error {
	event := domain.NewEvent(domain.EventAMLCheckFailed, &domain.AMLCheckFailedPayload{
		CheckID:      checkID,
		ErrorMessage: errorMessage,
	})

	if err := u.messageBus.Publish(ctx, domain.EventAMLCheckFailed, event); err != nil {
		u.logger.Errorw("failed to publish failed event", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to publish failed event: %w", err)
	}

	return fmt.Errorf("%s", errorMessage)
}
