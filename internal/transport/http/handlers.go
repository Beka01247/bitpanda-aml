package http

import (
	"context"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/application"
	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"github.com/Beka01247/bitpanda-aml/internal/infrastructure/token"
	"github.com/go-chi/chi"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

var (
	checksTotal      = expvar.NewInt("checks_total")
	checksSuccess    = expvar.NewInt("checks_success")
	checksFailed     = expvar.NewInt("checks_failed")
	checksProcessing = expvar.NewInt("checks_processing")
)

type Handlers struct {
	checkAddressUseCase *application.CheckAddressUseCase
	getStatusUseCase    *application.GetCheckStatusUseCase
	reportStorage       domain.ReportStorage
	tokenProvider       *token.HMACToken
	checkWaitSeconds    int
	apiURL              string
	logger              *zap.SugaredLogger
	validator           *validator.Validate
}

func NewHandlers(
	checkAddressUseCase *application.CheckAddressUseCase,
	getStatusUseCase *application.GetCheckStatusUseCase,
	reportStorage domain.ReportStorage,
	tokenProvider *token.HMACToken,
	checkWaitSeconds int,
	apiURL string,
	logger *zap.SugaredLogger,
) *Handlers {
	return &Handlers{
		checkAddressUseCase: checkAddressUseCase,
		getStatusUseCase:    getStatusUseCase,
		reportStorage:       reportStorage,
		tokenProvider:       tokenProvider,
		checkWaitSeconds:    checkWaitSeconds,
		apiURL:              apiURL,
		logger:              logger,
		validator:           validator.New(),
	}
}

// CheckAddress handles POST /v1/check-address
//
//	@Summary		Check cryptocurrency address
//	@Description	Initiates an AML check for a cryptocurrency address
//	@Tags			aml
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CheckAddressRequest	true	"Check request"
//	@Success		200		{object}	CheckAddressResponse
//	@Success		202		{object}	CheckAddressAcceptedResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Failure		502		{object}	ErrorResponse
//	@Router			/check-address [post]
func (h *Handlers) CheckAddress(w http.ResponseWriter, r *http.Request) {
	checksTotal.Add(1)

	var req CheckAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, fmt.Sprintf("validation failed: %v", err))
		return
	}

	// initiate check
	checkID, err := h.checkAddressUseCase.Execute(r.Context(), req.Address, req.Currency)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidAddress) || errors.Is(err, domain.ErrUnsupportedCurrency) {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.logger.Errorw("failed to initiate check", "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to initiate check")
		return
	}

	checksProcessing.Add(1)
	h.logger.Infow("check initiated", "check_id", checkID)

	// wait for completion (bounded wait)
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.checkWaitSeconds)*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// timeout - return 202 with poll URL
			checksProcessing.Add(-1)
			h.respondJSON(w, http.StatusAccepted, CheckAddressAcceptedResponse{
				Status:  "processing",
				Message: "Check is being processed. Use the poll_url to check status.",
				PollURL: fmt.Sprintf("%s/v1/check-address/%s", h.apiURL, checkID),
			})
			return

		case <-ticker.C:
			check, err := h.getStatusUseCase.Execute(r.Context(), checkID)
			if err != nil {
				h.logger.Warnw("failed to get check status", "check_id", checkID, "error", err)
				continue
			}

			if check.Status == domain.StatusCompleted {
				checksProcessing.Add(-1)
				checksSuccess.Add(1)
				h.respondCheckResult(w, check)
				return
			}

			if check.Status == domain.StatusFailed {
				checksProcessing.Add(-1)
				checksFailed.Add(1)
				h.respondError(w, http.StatusBadGateway, fmt.Sprintf("AML check failed: %s", check.ErrorMessage))
				return
			}
		}
	}
}

// GetCheckStatus handles GET /v1/check-address/{check_id}
//
//	@Summary		Get check status
//	@Description	Retrieves the status of an AML check
//	@Tags			aml
//	@Produce		json
//	@Param			check_id	path		string	true	"Check ID"
//	@Success		200			{object}	CheckAddressResponse
//	@Success		202			{object}	CheckAddressAcceptedResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/check-address/{check_id} [get]
func (h *Handlers) GetCheckStatus(w http.ResponseWriter, r *http.Request) {
	checkID := chi.URLParam(r, "check_id")
	if checkID == "" {
		h.respondError(w, http.StatusBadRequest, "check_id is required")
		return
	}

	check, err := h.getStatusUseCase.Execute(r.Context(), checkID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "check not found")
			return
		}
		if strings.Contains(err.Error(), "expired") {
			h.respondError(w, http.StatusGone, "check expired")
			return
		}
		h.logger.Errorw("failed to get check status", "check_id", checkID, "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to get check status")
		return
	}

	if check.Status == domain.StatusProcessing {
		h.respondJSON(w, http.StatusAccepted, CheckAddressAcceptedResponse{
			Status:  "processing",
			Message: "Check is being processed.",
			PollURL: fmt.Sprintf("%s/v1/check-address/%s", h.apiURL, checkID),
		})
		return
	}

	if check.Status == domain.StatusFailed {
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("check failed: %s", check.ErrorMessage))
		return
	}

	h.respondCheckResult(w, check)
}

// GetReport handles GET /v1/report/{token}.pdf
//
//	@Summary		Download report
//	@Description	Downloads or redirects to the PDF report
//	@Tags			aml
//	@Produce		application/pdf
//	@Param			token	path		string	true	"Report token"
//	@Success		200		{file}		binary
//	@Success		302		{string}	string	"Redirect to presigned URL"
//	@Failure		404		{object}	ErrorResponse
//	@Failure		410		{object}	ErrorResponse
//	@Router			/report/{token}.pdf [get]
func (h *Handlers) GetReport(w http.ResponseWriter, r *http.Request) {
	tokenStr := chi.URLParam(r, "token")
	if tokenStr == "" {
		h.respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	// verify token
	reportKey, err := h.tokenProvider.Verify(tokenStr)
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			h.respondError(w, http.StatusGone, "report link expired")
			return
		}
		h.respondError(w, http.StatusBadRequest, "invalid token")
		return
	}

	// try to get presigned URL first
	presignedURL, err := h.reportStorage.PresignGet(r.Context(), reportKey, 5*time.Minute)
	if err == nil && presignedURL != "" {
		// Redirect to presigned URL
		http.Redirect(w, r, presignedURL, http.StatusFound)
		return
	}

	// fallback: stream from storage
	data, err := h.reportStorage.Get(r.Context(), reportKey)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "report not found")
			return
		}
		if strings.Contains(err.Error(), "expired") {
			h.respondError(w, http.StatusGone, "report expired")
			return
		}
		h.logger.Errorw("failed to get report", "report_key", reportKey, "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to get report")
		return
	}

	// stream PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", reportKey))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (h *Handlers) respondCheckResult(w http.ResponseWriter, check *domain.AMLCheck) {
	// generate signed token for PDF URL
	token := h.tokenProvider.Sign(check.ReportKey, 24*time.Hour)
	pdfURL := fmt.Sprintf("%s/v1/report/%s", h.apiURL, token)

	// ensure categories is not nil
	categories := check.Categories
	if categories == nil {
		categories = []string{}
	}

	h.respondJSON(w, http.StatusOK, CheckAddressResponse{
		Status:     "success",
		RiskScore:  check.RiskScore,
		RiskLevel:  string(check.RiskLevel),
		Categories: categories,
		Sanctions:  ToSanctionsDTO(check.Sanctions),
		PDFURL:     pdfURL,
	})
}

func (h *Handlers) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handlers) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, ErrorResponse{Error: message})
}
