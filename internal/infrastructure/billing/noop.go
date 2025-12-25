package billing

import (
	"context"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type NoopBillingHook struct {
	logger *zap.SugaredLogger
}

func NewNoopBillingHook(logger *zap.SugaredLogger) *NoopBillingHook {
	return &NoopBillingHook{
		logger: logger,
	}
}

func (h *NoopBillingHook) OnCheckCompleted(ctx context.Context, check *domain.AMLCheck) error {
	h.logger.Debugw("billing hook (noop)", "check_id", check.ID, "risk_score", check.RiskScore)
	return nil
}
