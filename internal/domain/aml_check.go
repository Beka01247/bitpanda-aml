package domain

import (
	"time"

	"github.com/google/uuid"
)

type AMLCheckStatus string

const (
	StatusProcessing AMLCheckStatus = "processing"
	StatusCompleted  AMLCheckStatus = "completed"
	StatusFailed     AMLCheckStatus = "failed"
)

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "Low"
	RiskLevelMedium   RiskLevel = "Medium"
	RiskLevelHigh     RiskLevel = "High"
	RiskLevelCritical RiskLevel = "Critical"
)

func DeriveRiskLevel(score int) RiskLevel {
	if score >= 80 {
		return RiskLevelCritical
	} else if score >= 60 {
		return RiskLevelHigh
	} else if score >= 30 {
		return RiskLevelMedium
	}
	return RiskLevelLow
}

type SanctionsResult struct {
	Hit             bool                      `json:"hit"`
	Identifications []SanctionsIdentification `json:"identifications"`
}

type SanctionsIdentification struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	URL      string `json:"url"`
}

type AMLCheck struct {
	ID           string
	Address      string
	Currency     string
	Status       AMLCheckStatus
	RiskScore    int
	RiskLevel    RiskLevel
	Categories   []string
	Sanctions    *SanctionsResult
	ReportKey    string
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ExpiresAt    time.Time
}

func NewAMLCheck(address, currency string, ttl time.Duration) *AMLCheck {
	now := time.Now().UTC()
	return &AMLCheck{
		ID:         uuid.New().String(),
		Address:    address,
		Currency:   currency,
		Status:     StatusProcessing,
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  now.Add(ttl),
		Sanctions:  &SanctionsResult{Hit: false, Identifications: []SanctionsIdentification{}},
		Categories: []string{},
	}
}

func (c *AMLCheck) MarkCompleted(riskScore int, riskLevel RiskLevel, categories []string, sanctions *SanctionsResult, reportKey string) {
	c.Status = StatusCompleted
	c.RiskScore = riskScore
	c.RiskLevel = riskLevel
	c.Categories = categories
	c.Sanctions = sanctions
	c.ReportKey = reportKey
	c.UpdatedAt = time.Now().UTC()
}

func (c *AMLCheck) MarkFailed(errorMessage string) {
	c.Status = StatusFailed
	c.ErrorMessage = errorMessage
	c.UpdatedAt = time.Now().UTC()
}

func (c *AMLCheck) IsExpired() bool {
	return time.Now().UTC().After(c.ExpiresAt)
}
