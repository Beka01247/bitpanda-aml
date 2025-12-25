package domain

import (
	"testing"
)

func TestBitcoinValidateAddress(t *testing.T) {
	btc := Bitcoin{}

	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"valid legacy", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", false},
		{"valid legacy 3", "3J98t1WpEZ73CNmYviecrnyiWrnqRhWNLy", false},
		{"valid bech32", "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq", false},
		{"empty", "", true},
		{"invalid prefix", "2A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", true},
		{"too short", "1A1zP", true},
		{"invalid chars", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfN@", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := btc.ValidateAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bitcoin.ValidateAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEthereumValidateAddress(t *testing.T) {
	eth := Ethereum{}

	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"valid lowercase", "0x742d35cc6634c0532925a3b844bc9e7595f0beb8", false},
		{"valid uppercase", "0x742D35CC6634C0532925A3B844BC9E7595F0BEB8", false},
		{"valid mixed", "0x742d35Cc6634C0532925a3b844Bc9e7595f0beb8", false},
		{"empty", "", true},
		{"missing 0x", "742d35cc6634c0532925a3b844bc9e7595f0beb8", true},
		{"too short", "0x742d35cc", true},
		{"invalid char", "0x742d35cc6634c0532925a3b844bc9e7595f0beg8", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eth.ValidateAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("Ethereum.ValidateAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUSDTValidateAddress(t *testing.T) {
	usdt := USDT{}

	// USDT on Ethereum uses Ethereum address format
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"valid eth address", "0x742d35cc6634c0532925a3b844bc9e7595f0beb8", false},
		{"empty", "", true},
		{"invalid format", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := usdt.ValidateAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("USDT.ValidateAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEthereumNormalizeAddress(t *testing.T) {
	eth := Ethereum{}

	tests := []struct {
		name    string
		address string
		want    string
	}{
		{"uppercase", "0x742D35CC6634C0532925A3B844BC9E7595F0BEB8", "0x742d35cc6634c0532925a3b844bc9e7595f0beb8"},
		{"mixed", "0x742d35Cc6634C0532925a3b844Bc9e7595f0beb8", "0x742d35cc6634c0532925a3b844bc9e7595f0beb8"},
		{"with spaces", " 0x742d35cc6634c0532925a3b844bc9e7595f0beb8 ", "0x742d35cc6634c0532925a3b844bc9e7595f0beb8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eth.NormalizeAddress(tt.address)
			if got != tt.want {
				t.Errorf("Ethereum.NormalizeAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultAssetRegistry(t *testing.T) {
	registry := NewDefaultAssetRegistry()

	t.Run("get supported currencies", func(t *testing.T) {
		currencies := []string{"BTC", "ETH", "USDT"}
		for _, currency := range currencies {
			asset, err := registry.Get(currency)
			if err != nil {
				t.Errorf("registry.Get(%s) error = %v", currency, err)
			}
			if asset == nil {
				t.Errorf("registry.Get(%s) returned nil", currency)
			}
		}
	})

	t.Run("get unsupported currency", func(t *testing.T) {
		_, err := registry.Get("XRP")
		if err == nil {
			t.Error("registry.Get(XRP) should return error")
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		asset, err := registry.Get("btc")
		if err != nil {
			t.Errorf("registry.Get(btc) error = %v", err)
		}
		if asset.Symbol() != "BTC" {
			t.Errorf("registry.Get(btc) symbol = %v, want BTC", asset.Symbol())
		}
	})

	t.Run("list all", func(t *testing.T) {
		assets := registry.List()
		if len(assets) != 3 {
			t.Errorf("registry.List() length = %v, want 3", len(assets))
		}
	})
}


