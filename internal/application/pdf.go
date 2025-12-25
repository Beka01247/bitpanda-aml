package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"github.com/jung-kurt/gofpdf"
)

func GeneratePDF(address, currency string, riskScore int, riskLevel domain.RiskLevel, categories []string, sanctions *domain.SanctionsResult, checkID string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 20)
	pdf.Cell(0, 10, "AML Check Report")
	pdf.Ln(15)

	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Generated: %s UTC", time.Now().UTC().Format("2006-01-02 15:04:05")))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Address Information")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(40, 6, "Address:")
	pdf.SetFont("Arial", "B", 11)
	pdf.MultiCell(0, 6, address, "", "", false)
	pdf.Ln(2)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(40, 6, "Currency:")
	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(0, 6, currency)
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Risk Assessment")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(40, 6, "Risk Score:")
	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(0, 6, fmt.Sprintf("%d / 100", riskScore))
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(40, 6, "Risk Level:")
	pdf.SetFont("Arial", "B", 11)

	switch riskLevel {
	case domain.RiskLevelLow:
		pdf.SetTextColor(0, 128, 0)
	case domain.RiskLevelMedium:
		pdf.SetTextColor(255, 165, 0)
	case domain.RiskLevelHigh:
		pdf.SetTextColor(255, 0, 0)
	case domain.RiskLevelCritical:
		pdf.SetTextColor(139, 0, 0)
	}
	pdf.Cell(0, 6, string(riskLevel))
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(10)

	if len(categories) > 0 {
		pdf.SetFont("Arial", "", 11)
		pdf.Cell(40, 6, "Categories:")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 10)
		for _, category := range categories {
			pdf.Cell(10, 5, "")
			pdf.Cell(0, 5, fmt.Sprintf("- %s", category))
			pdf.Ln(5)
		}
	} else {
		pdf.SetFont("Arial", "", 11)
		pdf.Cell(40, 6, "Categories:")
		pdf.SetFont("Arial", "I", 10)
		pdf.Cell(0, 6, "None")
		pdf.Ln(6)
	}
	pdf.Ln(5)

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "Sanctions Screening (Chainalysis)")
	pdf.Ln(8)

	if sanctions != nil && sanctions.Hit {
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(255, 0, 0)
		pdf.Cell(0, 6, "SANCTIONS DETECTED")
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(8)

		for _, identification := range sanctions.Identifications {
			pdf.SetFont("Arial", "B", 10)
			pdf.Cell(0, 5, fmt.Sprintf("Category: %s", identification.Category))
			pdf.Ln(5)

			pdf.SetFont("Arial", "", 10)
			pdf.Cell(10, 5, "")
			pdf.Cell(20, 5, "Name:")
			pdf.MultiCell(0, 5, identification.Name, "", "", false)

			if identification.URL != "" {
				pdf.SetFont("Arial", "I", 9)
				pdf.SetTextColor(0, 0, 255)
				pdf.Cell(10, 5, "")
				pdf.Cell(20, 5, "URL:")
				displayURL := identification.URL
				if len(displayURL) > 80 {
					displayURL = displayURL[:80] + "..."
				}
				pdf.Cell(0, 5, displayURL)
				pdf.SetTextColor(0, 0, 0)
				pdf.Ln(5)
			}
			pdf.Ln(3)
		}
	} else {
		pdf.SetFont("Arial", "", 11)
		pdf.SetTextColor(0, 128, 0)
		pdf.Cell(0, 6, "No sanctions detected")
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(6)
	}

	pdf.SetY(-20)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(128, 128, 128)
	pdf.Cell(0, 10, fmt.Sprintf("Check ID: %s", checkID))

	// generate PDF bytes
	var buf strings.Builder
	err := pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate pdf output: %w", err)
	}

	return []byte(buf.String()), nil
}
