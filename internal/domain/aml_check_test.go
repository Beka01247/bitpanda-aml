package domain

import (
	"testing"
	"time"
)

func TestDeriveRiskLevel(t *testing.T) {
	tests := []struct {
		name  string
		score int
		want  RiskLevel
	}{
		{"critical high", 90, RiskLevelCritical},
		{"critical boundary", 80, RiskLevelCritical},
		{"high", 75, RiskLevelHigh},
		{"high boundary", 60, RiskLevelHigh},
		{"medium", 50, RiskLevelMedium},
		{"medium boundary", 30, RiskLevelMedium},
		{"low", 20, RiskLevelLow},
		{"very low", 5, RiskLevelLow},
		{"zero", 0, RiskLevelLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveRiskLevel(tt.score)
			if got != tt.want {
				t.Errorf("DeriveRiskLevel(%d) = %v, want %v", tt.score, got, tt.want)
			}
		})
	}
}

func TestNewAMLCheck(t *testing.T) {
	address := "0x742d35cc6634c0532925a3b844bc9e7595f0beb8"
	currency := "ETH"
	ttl := 24 * time.Hour

	check := NewAMLCheck(address, currency, ttl)

	if check.ID == "" {
		t.Error("NewAMLCheck() ID is empty")
	}

	if check.Address != address {
		t.Errorf("NewAMLCheck() Address = %v, want %v", check.Address, address)
	}

	if check.Currency != currency {
		t.Errorf("NewAMLCheck() Currency = %v, want %v", check.Currency, currency)
	}

	if check.Status != StatusProcessing {
		t.Errorf("NewAMLCheck() Status = %v, want %v", check.Status, StatusProcessing)
	}

	if check.Sanctions == nil {
		t.Error("NewAMLCheck() Sanctions is nil")
	}

	if check.Sanctions.Hit {
		t.Error("NewAMLCheck() Sanctions.Hit should be false")
	}

	if len(check.Sanctions.Identifications) != 0 {
		t.Error("NewAMLCheck() Sanctions.Identifications should be empty")
	}

	if check.ExpiresAt.Before(time.Now().UTC()) {
		t.Error("NewAMLCheck() ExpiresAt is in the past")
	}
}

func TestAMLCheck_MarkCompleted(t *testing.T) {
	check := NewAMLCheck("test-address", "BTC", time.Hour)

	riskScore := 85
	riskLevel := RiskLevelHigh
	categories := []string{"Darknet", "Mixer"}
	sanctions := &SanctionsResult{
		Hit: true,
		Identifications: []SanctionsIdentification{
			{Category: "sanctions", Name: "Test", URL: "https://test.com"},
		},
	}
	reportKey := "test-report.pdf"

	check.MarkCompleted(riskScore, riskLevel, categories, sanctions, reportKey)

	if check.Status != StatusCompleted {
		t.Errorf("MarkCompleted() Status = %v, want %v", check.Status, StatusCompleted)
	}

	if check.RiskScore != riskScore {
		t.Errorf("MarkCompleted() RiskScore = %v, want %v", check.RiskScore, riskScore)
	}

	if check.RiskLevel != riskLevel {
		t.Errorf("MarkCompleted() RiskLevel = %v, want %v", check.RiskLevel, riskLevel)
	}

	if len(check.Categories) != len(categories) {
		t.Errorf("MarkCompleted() Categories length = %v, want %v", len(check.Categories), len(categories))
	}

	if check.Sanctions != sanctions {
		t.Error("MarkCompleted() Sanctions not set correctly")
	}

	if check.ReportKey != reportKey {
		t.Errorf("MarkCompleted() ReportKey = %v, want %v", check.ReportKey, reportKey)
	}
}

func TestAMLCheck_MarkFailed(t *testing.T) {
	check := NewAMLCheck("test-address", "BTC", time.Hour)
	errorMessage := "test error"

	check.MarkFailed(errorMessage)

	if check.Status != StatusFailed {
		t.Errorf("MarkFailed() Status = %v, want %v", check.Status, StatusFailed)
	}

	if check.ErrorMessage != errorMessage {
		t.Errorf("MarkFailed() ErrorMessage = %v, want %v", check.ErrorMessage, errorMessage)
	}
}

func TestAMLCheck_IsExpired(t *testing.T) {
	t.Run("not expired", func(t *testing.T) {
		check := NewAMLCheck("test-address", "BTC", time.Hour)
		if check.IsExpired() {
			t.Error("IsExpired() = true, want false")
		}
	})

	t.Run("expired", func(t *testing.T) {
		check := NewAMLCheck("test-address", "BTC", -time.Hour)
		if !check.IsExpired() {
			t.Error("IsExpired() = false, want true")
		}
	})
}


