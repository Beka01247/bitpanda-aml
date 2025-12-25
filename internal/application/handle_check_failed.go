package application

import (
	"context"
	"fmt"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type HandleCheckFailedUseCase struct {
	repository domain.AMLCheckRepository
	logger     *zap.SugaredLogger
}

func NewHandleCheckFailedUseCase(
	repository domain.AMLCheckRepository,
	logger *zap.SugaredLogger,
) *HandleCheckFailedUseCase {
	return &HandleCheckFailedUseCase{
		repository: repository,
		logger:     logger,
	}
}

// executes the handle check failed use case
func (u *HandleCheckFailedUseCase) Execute(ctx context.Context, checkID, errorMessage string) error {
	u.logger.Warnw("handling check failure", "check_id", checkID, "error", errorMessage)

	check, err := u.repository.Get(ctx, checkID)
	if err != nil {
		u.logger.Errorw("failed to get check", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to get check: %w", err)
	}

	if check == nil {
		return fmt.Errorf("check not found")
	}

	check.MarkFailed(errorMessage)
	if err := u.repository.Update(ctx, check); err != nil {
		u.logger.Errorw("failed to update check", "check_id", checkID, "error", err)
		return fmt.Errorf("failed to update check: %w", err)
	}

	return nil
}
