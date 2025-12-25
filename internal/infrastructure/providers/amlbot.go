package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type AMLBotProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

type AMLBotResponse struct {
	RiskScore  int      `json:"risk_score"`
	RiskLevel  string   `json:"risk_level,omitempty"`
	Categories []string `json:"categories"`
}

func NewAMLBotProvider(baseURL, apiKey string, logger *zap.SugaredLogger) *AMLBotProvider {
	return &AMLBotProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (p *AMLBotProvider) CheckAddress(ctx context.Context, address, currency string) (*domain.AMLResult, error) {
	url := fmt.Sprintf("%s/check?address=%s&currency=%s", p.baseURL, address, currency)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("amlbot returned status %d: %s", resp.StatusCode, string(body))
	}

	var amlbotResp AMLBotResponse
	if err := json.NewDecoder(resp.Body).Decode(&amlbotResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// derive risk level if not provided
	riskLevel := domain.RiskLevel(amlbotResp.RiskLevel)
	if riskLevel == "" {
		riskLevel = domain.DeriveRiskLevel(amlbotResp.RiskScore)
	}

	// ensure categories is not nil
	categories := amlbotResp.Categories
	if categories == nil {
		categories = []string{}
	}

	return &domain.AMLResult{
		RiskScore:  amlbotResp.RiskScore,
		RiskLevel:  riskLevel,
		Categories: categories,
	}, nil
}

func (p *AMLBotProvider) Name() string {
	return "AMLBot"
}
