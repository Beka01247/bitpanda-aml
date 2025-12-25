package providers

import (
	"context"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type MockAMLProvider struct {
	logger *zap.SugaredLogger
}

func NewMockAMLProvider(logger *zap.SugaredLogger) *MockAMLProvider {
	return &MockAMLProvider{
		logger: logger,
	}
}

func (p *MockAMLProvider) CheckAddress(ctx context.Context, address, currency string) (*domain.AMLResult, error) {
	p.logger.Infow("mock aml check", "address", address, "currency", currency)

	// generate deterministic-ish risk score based on address
	score := 10 + (len(address) % 80)

	riskLevel := domain.DeriveRiskLevel(score)

	categories := []string{}
	if score >= 60 {
		categories = append(categories, "High Risk Exchange")
	}
	if score >= 80 {
		categories = append(categories, "Darknet", "Mixer")
	}

	return &domain.AMLResult{
		RiskScore:  score,
		RiskLevel:  riskLevel,
		Categories: categories,
	}, nil
}

func (p *MockAMLProvider) Name() string {
	return "MockAML"
}
