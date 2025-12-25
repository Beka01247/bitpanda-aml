package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidAddress      = errors.New("invalid address format")
	ErrUnsupportedCurrency = errors.New("unsupported currency")
)

// represents a cryptocurrency asset
type Asset interface {
	Symbol() string
	Chain() string
	ValidateAddress(address string) error
	NormalizeAddress(address string) string
}

// manages supported assets
type AssetRegistry interface {
	Get(symbol string) (Asset, error)
	List() []Asset
}

// btc implementation
type Bitcoin struct{}

func (b Bitcoin) Symbol() string { return "BTC" }
func (b Bitcoin) Chain() string  { return "bitcoin" }

func (b Bitcoin) ValidateAddress(address string) error {
	if address == "" {
		return ErrInvalidAddress
	}
	// base58 (legacy): starts with 1 or 3, length 26-35
	// bech32 (native segwit): starts with bc1, length 42-62
	if (strings.HasPrefix(address, "1") || strings.HasPrefix(address, "3")) && len(address) >= 26 && len(address) <= 35 {
		matched, _ := regexp.MatchString(`^[13][a-km-zA-HJ-NP-Z1-9]{25,34}$`, address)
		if matched {
			return nil
		}
	}
	if strings.HasPrefix(address, "bc1") && len(address) >= 42 && len(address) <= 62 {
		matched, _ := regexp.MatchString(`^bc1[a-z0-9]{39,59}$`, address)
		if matched {
			return nil
		}
	}
	return ErrInvalidAddress
}

func (b Bitcoin) NormalizeAddress(address string) string {
	return strings.TrimSpace(address)
}

// eth implementation
type Ethereum struct{}

func (e Ethereum) Symbol() string { return "ETH" }
func (e Ethereum) Chain() string  { return "ethereum" }

func (e Ethereum) ValidateAddress(address string) error {
	if address == "" {
		return ErrInvalidAddress
	}
	// eth addresses: 0x + 40 hex chars
	matched, _ := regexp.MatchString(`^0x[a-fA-F0-9]{40}$`, address)
	if !matched {
		return ErrInvalidAddress
	}
	return nil
}

func (e Ethereum) NormalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

type USDT struct{}

func (u USDT) Symbol() string { return "USDT" }
func (u USDT) Chain() string  { return "ethereum" }

func (u USDT) ValidateAddress(address string) error {
	eth := Ethereum{}
	return eth.ValidateAddress(address)
}

func (u USDT) NormalizeAddress(address string) string {
	eth := Ethereum{}
	return eth.NormalizeAddress(address)
}

type DefaultAssetRegistry struct {
	assets map[string]Asset
}

func NewDefaultAssetRegistry() *DefaultAssetRegistry {
	registry := &DefaultAssetRegistry{
		assets: make(map[string]Asset),
	}
	registry.register(Bitcoin{})
	registry.register(Ethereum{})
	registry.register(USDT{})
	return registry
}

func (r *DefaultAssetRegistry) register(asset Asset) {
	r.assets[asset.Symbol()] = asset
}

func (r *DefaultAssetRegistry) Get(symbol string) (Asset, error) {
	asset, ok := r.assets[strings.ToUpper(symbol)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCurrency, symbol)
	}
	return asset, nil
}

func (r *DefaultAssetRegistry) List() []Asset {
	assets := make([]Asset, 0, len(r.assets))
	for _, asset := range r.assets {
		assets = append(assets, asset)
	}
	return assets
}
