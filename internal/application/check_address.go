package application

import (
	"context"
	"fmt"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"go.uber.org/zap"
)

type CheckAddressUseCase struct {
	assetRegistry domain.AssetRegistry
	repository    domain.AMLCheckRepository
	messageBus    domain.MessageBus
	checkTTL      time.Duration
	logger        *zap.SugaredLogger
}

func NewCheckAddressUseCase(
	assetRegistry domain.AssetRegistry,
	repository domain.AMLCheckRepository,
	messageBus domain.MessageBus,
	checkTTL time.Duration,
	logger *zap.SugaredLogger,
) *CheckAddressUseCase {
	return &CheckAddressUseCase{
		assetRegistry: assetRegistry,
		repository:    repository,
		messageBus:    messageBus,
		checkTTL:      checkTTL,
		logger:        logger,
	}
}

// executes the check address use case
func (u *CheckAddressUseCase) Execute(ctx context.Context, address, currency string) (string, error) {
	// validate currency and address
	asset, err := u.assetRegistry.Get(currency)
	if err != nil {
		return "", err
	}

	normalizedAddress := asset.NormalizeAddress(address)
	if err := asset.ValidateAddress(normalizedAddress); err != nil {
		return "", fmt.Errorf("invalid address: %w", err)
	}

	// create AML check
	check := domain.NewAMLCheck(normalizedAddress, asset.Symbol(), u.checkTTL)

	// persist state
	if err := u.repository.Create(ctx, check); err != nil {
		u.logger.Errorw("failed to create check", "check_id", check.ID, "error", err)
		return "", fmt.Errorf("failed to create check: %w", err)
	}

	// publish event
	event := domain.NewEvent(domain.EventAMLCheckRequested, &domain.AMLCheckRequestedPayload{
		CheckID:  check.ID,
		Address:  normalizedAddress,
		Currency: asset.Symbol(),
	})

	if err := u.messageBus.Publish(ctx, domain.EventAMLCheckRequested, event); err != nil {
		u.logger.Errorw("failed to publish event", "check_id", check.ID, "error", err)
		return "", fmt.Errorf("failed to publish event: %w", err)
	}

	u.logger.Infow("check initiated", "check_id", check.ID, "address", normalizedAddress, "currency", currency)

	return check.ID, nil
}
