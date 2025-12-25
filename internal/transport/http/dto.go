package http

import "github.com/Beka01247/bitpanda-aml/internal/domain"

type CheckAddressRequest struct {
	Address  string `json:"address" validate:"required"`
	Currency string `json:"currency" validate:"required,oneof=BTC ETH USDT"`
}

type CheckAddressResponse struct {
	Status     string               `json:"status"`
	RiskScore  int                  `json:"risk_score"`
	RiskLevel  string               `json:"risk_level"`
	Categories []string             `json:"categories"`
	Sanctions  SanctionsResponseDTO `json:"sanctions"`
	PDFURL     string               `json:"pdf_url"`
}

type CheckAddressAcceptedResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	PollURL string `json:"poll_url"`
}

type SanctionsResponseDTO struct {
	Hit             bool                         `json:"hit"`
	Identifications []SanctionsIdentificationDTO `json:"identifications"`
}

type SanctionsIdentificationDTO struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	URL      string `json:"url"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func ToSanctionsDTO(sanctions *domain.SanctionsResult) SanctionsResponseDTO {
	if sanctions == nil {
		return SanctionsResponseDTO{
			Hit:             false,
			Identifications: []SanctionsIdentificationDTO{},
		}
	}

	identifications := make([]SanctionsIdentificationDTO, 0, len(sanctions.Identifications))
	for _, ident := range sanctions.Identifications {
		identifications = append(identifications, SanctionsIdentificationDTO{
			Category: ident.Category,
			Name:     ident.Name,
			URL:      ident.URL,
		})
	}

	return SanctionsResponseDTO{
		Hit:             sanctions.Hit,
		Identifications: identifications,
	}
}
