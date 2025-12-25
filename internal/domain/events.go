package domain

import (
	"encoding/json"
	"time"
)

// event types
const (
	EventAMLCheckRequested = "aml.check.requested"
	EventAMLCheckCompleted = "aml.check.completed"
	EventAMLReportReady    = "aml.report.ready"
	EventAMLCheckFailed    = "aml.check.failed"
)

type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

type AMLCheckRequestedPayload struct {
	CheckID  string `json:"check_id"`
	Address  string `json:"address"`
	Currency string `json:"currency"`
}

type AMLCheckCompletedPayload struct {
	CheckID    string           `json:"check_id"`
	RiskScore  int              `json:"risk_score"`
	RiskLevel  RiskLevel        `json:"risk_level"`
	Categories []string         `json:"categories"`
	Sanctions  *SanctionsResult `json:"sanctions"`
}

type AMLReportReadyPayload struct {
	CheckID   string `json:"check_id"`
	ReportKey string `json:"report_key"`
}

type AMLCheckFailedPayload struct {
	CheckID      string `json:"check_id"`
	ErrorMessage string `json:"error_message"`
}

func NewEvent(eventType string, payload any) *Event {
	return &Event{
		ID:        time.Now().Format("20060102150405.000000"),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
}

func (e *Event) MarshalPayload() ([]byte, error) {
	return json.Marshal(e.Payload)
}
