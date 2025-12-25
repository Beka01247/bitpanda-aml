package domain

import (
	"context"
	"time"
)

type AMLProvider interface {
	CheckAddress(ctx context.Context, address, currency string) (*AMLResult, error)
	Name() string
}

type AMLResult struct {
	RiskScore  int
	RiskLevel  RiskLevel
	Categories []string
}

type SanctionsProvider interface {
	CheckAddress(ctx context.Context, address string) (*SanctionsResult, error)
	Name() string
}

type MessageBus interface {
	Publish(ctx context.Context, routingKey string, event *Event) error
	Subscribe(ctx context.Context, queueName string, routingKeys []string, handler func([]byte) error) error
	Close() error
}

type AMLCheckRepository interface {
	Create(ctx context.Context, check *AMLCheck) error
	Get(ctx context.Context, checkID string) (*AMLCheck, error)
	Update(ctx context.Context, check *AMLCheck) error
	CleanupExpired(ctx context.Context, now time.Time) (int, error)
}

type ReportStorage interface {
	Put(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	PresignGet(ctx context.Context, key string, expires time.Duration) (string, error)
	CleanupExpired(ctx context.Context, now time.Time) (int, error)
}

type BillingHook interface {
	OnCheckCompleted(ctx context.Context, check *AMLCheck) error
}
