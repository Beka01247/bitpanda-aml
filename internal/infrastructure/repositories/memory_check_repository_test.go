package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

func TestMemoryCheckRepository_CleanupExpired(t *testing.T) {
	logger := zap.NewNop().Sugar()
	repo := NewMemoryCheckRepository(logger)
	ctx := context.Background()

	// create checks with different TTLs
	check1 := domain.NewAMLCheck("address1", "BTC", -1*time.Hour)    // Already expired
	check2 := domain.NewAMLCheck("address2", "ETH", time.Hour)       // Not expired
	check3 := domain.NewAMLCheck("address3", "USDT", -1*time.Minute) // Already expired

	if err := repo.Create(ctx, check1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Create(ctx, check2); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Create(ctx, check3); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// run cleanup
	count, err := repo.CleanupExpired(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}

	if count != 2 {
		t.Errorf("CleanupExpired() count = %v, want 2", count)
	}

	// verify expired checks are removed
	if check, _ := repo.Get(ctx, check1.ID); check != nil {
		t.Error("Expired check1 should be removed")
	}
	if check, _ := repo.Get(ctx, check3.ID); check != nil {
		t.Error("Expired check3 should be removed")
	}

	// verify non-expired check still exists
	check, err := repo.Get(ctx, check2.ID)
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if check == nil {
		t.Error("Non-expired check2 should still exist")
	}
}

func TestMemoryCheckRepository_CRUD(t *testing.T) {
	logger := zap.NewNop().Sugar()
	repo := NewMemoryCheckRepository(logger)
	ctx := context.Background()

	t.Run("create and get", func(t *testing.T) {
		check := domain.NewAMLCheck("test-address", "BTC", time.Hour)

		err := repo.Create(ctx, check)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		retrieved, err := repo.Get(ctx, check.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved.ID != check.ID {
			t.Errorf("Get() ID = %v, want %v", retrieved.ID, check.ID)
		}
	})

	t.Run("duplicate create", func(t *testing.T) {
		check := domain.NewAMLCheck("test-address", "BTC", time.Hour)

		err := repo.Create(ctx, check)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		err = repo.Create(ctx, check)
		if err == nil {
			t.Error("Create() should return error for duplicate")
		}
	})

	t.Run("update", func(t *testing.T) {
		check := domain.NewAMLCheck("test-address", "BTC", time.Hour)

		err := repo.Create(ctx, check)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		check.MarkCompleted(75, domain.RiskLevelHigh, []string{"Test"}, &domain.SanctionsResult{Hit: false, Identifications: []domain.SanctionsIdentification{}}, "report.pdf")

		err = repo.Update(ctx, check)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		retrieved, err := repo.Get(ctx, check.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if retrieved.Status != domain.StatusCompleted {
			t.Errorf("Get() Status = %v, want %v", retrieved.Status, domain.StatusCompleted)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		check, err := repo.Get(ctx, "non-existent")
		if err != nil {
			t.Errorf("Get() error = %v", err)
		}
		if check != nil {
			t.Error("Get() should return nil for non-existent ID")
		}
	})
}
