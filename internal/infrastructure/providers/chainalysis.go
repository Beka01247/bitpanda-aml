package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type ChainalysisProvider struct {
	apiKey     string
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

type ChainalysisResponse struct {
	Identifications []ChainalysisIdentification `json:"identifications"`
}

type ChainalysisIdentification struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func NewChainalysisProvider(apiKey string, logger *zap.SugaredLogger) *ChainalysisProvider {
	return &ChainalysisProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (p *ChainalysisProvider) CheckAddress(ctx context.Context, address string) (*domain.SanctionsResult, error) {
	if p.apiKey == "" {
		p.logger.Warn("chainalysis api key not set, returning empty result")
		return &domain.SanctionsResult{
			Hit:             false,
			Identifications: []domain.SanctionsIdentification{},
		}, nil
	}

	url := fmt.Sprintf("https://public.chainalysis.com/api/v1/address/%s", address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// handle 404 as no sanctions found
	if resp.StatusCode == http.StatusNotFound {
		return &domain.SanctionsResult{
			Hit:             false,
			Identifications: []domain.SanctionsIdentification{},
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chainalysis returned status %d: %s", resp.StatusCode, string(body))
	}

	var chainResp ChainalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&chainResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// normalize identifications
	identifications := make([]domain.SanctionsIdentification, 0, len(chainResp.Identifications))
	for _, ident := range chainResp.Identifications {
		identifications = append(identifications, domain.SanctionsIdentification{
			Category: ident.Category,
			Name:     ident.Name,
			URL:      ident.URL,
		})
	}

	return &domain.SanctionsResult{
		Hit:             len(identifications) > 0,
		Identifications: identifications,
	}, nil
}

func (p *ChainalysisProvider) Name() string {
	return "Chainalysis"
}
