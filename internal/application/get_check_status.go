package application

import (
	"context"
	"fmt"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type GetCheckStatusUseCase struct {
	repository domain.AMLCheckRepository
	logger     *zap.SugaredLogger
}

func NewGetCheckStatusUseCase(
	repository domain.AMLCheckRepository,
	logger *zap.SugaredLogger,
) *GetCheckStatusUseCase {
	return &GetCheckStatusUseCase{
		repository: repository,
		logger:     logger,
	}
}

// executes the get check status use case
func (u *GetCheckStatusUseCase) Execute(ctx context.Context, checkID string) (*domain.AMLCheck, error) {
	check, err := u.repository.Get(ctx, checkID)
	if err != nil {
		u.logger.Errorw("failed to get check", "check_id", checkID, "error", err)
		return nil, fmt.Errorf("failed to get check: %w", err)
	}

	if check == nil {
		return nil, fmt.Errorf("check not found")
	}

	if check.IsExpired() {
		return nil, fmt.Errorf("check expired")
	}

	return check, nil
}
