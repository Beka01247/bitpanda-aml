package repositories

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type MemoryCheckRepository struct {
	checks map[string]*domain.AMLCheck
	mu     sync.RWMutex
	logger *zap.SugaredLogger
}

func NewMemoryCheckRepository(logger *zap.SugaredLogger) *MemoryCheckRepository {
	return &MemoryCheckRepository{
		checks: make(map[string]*domain.AMLCheck),
		logger: logger,
	}
}

func (r *MemoryCheckRepository) Create(ctx context.Context, check *domain.AMLCheck) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.checks[check.ID]; exists {
		return fmt.Errorf("check already exists")
	}

	r.checks[check.ID] = check
	r.logger.Debugw("check created", "check_id", check.ID)

	return nil
}

func (r *MemoryCheckRepository) Get(ctx context.Context, checkID string) (*domain.AMLCheck, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	check, exists := r.checks[checkID]
	if !exists {
		return nil, nil
	}

	return check, nil
}

func (r *MemoryCheckRepository) Update(ctx context.Context, check *domain.AMLCheck) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.checks[check.ID]; !exists {
		return fmt.Errorf("check not found")
	}

	r.checks[check.ID] = check
	r.logger.Debugw("check updated", "check_id", check.ID, "status", check.Status)

	return nil
}

// removes expired checks
func (r *MemoryCheckRepository) CleanupExpired(ctx context.Context, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for id, check := range r.checks {
		if now.After(check.ExpiresAt) {
			delete(r.checks, id)
			count++
		}
	}

	if count > 0 {
		r.logger.Infow("expired checks cleaned", "count", count)
	}

	return count, nil
}

// starts a background cleanup loop
func (r *MemoryCheckRepository) StartCleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				r.logger.Info("cleanup loop stopped")
				return
			case <-ticker.C:
				_, err := r.CleanupExpired(ctx, time.Now().UTC())
				if err != nil {
					r.logger.Errorw("cleanup failed", "error", err)
				}
			}
		}
	}()
}
