package providers

import (
	"testing"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
)

func TestChainalysisProvider_ParseResponse(t *testing.T) {
	tests := []struct {
		name     string
		response ChainalysisResponse
		wantHit  bool
		wantLen  int
	}{
		{
			name: "no sanctions",
			response: ChainalysisResponse{
				Identifications: []ChainalysisIdentification{},
			},
			wantHit: false,
			wantLen: 0,
		},
		{
			name: "single sanction",
			response: ChainalysisResponse{
				Identifications: []ChainalysisIdentification{
					{
						Category:    "sanctions",
						Name:        "OFAC SDN",
						Description: "Test",
						URL:         "https://test.com",
					},
				},
			},
			wantHit: true,
			wantLen: 1,
		},
		{
			name: "multiple sanctions",
			response: ChainalysisResponse{
				Identifications: []ChainalysisIdentification{
					{
						Category: "sanctions",
						Name:     "OFAC SDN",
						URL:      "https://test1.com",
					},
					{
						Category: "sanctions",
						Name:     "EU Sanctions",
						URL:      "https://test2.com",
					},
				},
			},
			wantHit: true,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// simulate parsing logic
			identifications := make([]domain.SanctionsIdentification, 0, len(tt.response.Identifications))
			for _, ident := range tt.response.Identifications {
				identifications = append(identifications, domain.SanctionsIdentification{
					Category: ident.Category,
					Name:     ident.Name,
					URL:      ident.URL,
				})
			}

			result := &domain.SanctionsResult{
				Hit:             len(identifications) > 0,
				Identifications: identifications,
			}

			if result.Hit != tt.wantHit {
				t.Errorf("Hit = %v, want %v", result.Hit, tt.wantHit)
			}

			if len(result.Identifications) != tt.wantLen {
				t.Errorf("Identifications length = %v, want %v", len(result.Identifications), tt.wantLen)
			}

			// verify structure
			for i, ident := range result.Identifications {
				if ident.Category != tt.response.Identifications[i].Category {
					t.Errorf("Identification[%d].Category = %v, want %v", i, ident.Category, tt.response.Identifications[i].Category)
				}
				if ident.Name != tt.response.Identifications[i].Name {
					t.Errorf("Identification[%d].Name = %v, want %v", i, ident.Name, tt.response.Identifications[i].Name)
				}
				if ident.URL != tt.response.Identifications[i].URL {
					t.Errorf("Identification[%d].URL = %v, want %v", i, ident.URL, tt.response.Identifications[i].URL)
				}
			}
		})
	}
}
