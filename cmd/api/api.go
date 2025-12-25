package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Beka01247/bitpanda-aml/docs"
	"github.com/Beka01247/bitpanda-aml/internal/env"
	"github.com/Beka01247/bitpanda-aml/internal/ratelimiter"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"
)

type application struct {
	config      config
	logger      *zap.SugaredLogger
	rateLimiter ratelimiter.Limiter
	handlers    interface {
		CheckAddress(w http.ResponseWriter, r *http.Request)
		GetCheckStatus(w http.ResponseWriter, r *http.Request)
		GetReport(w http.ResponseWriter, r *http.Request)
	}
}

type objectStorageConfig struct {
	endpoint  string
	publicURL string
	accessKey string
	secretKey string
	bucket    string
	useSSL    bool
}

type config struct {
	addr                 string
	env                  string
	apiURL               string
	frontendURL          string
	rateLimiter          ratelimiter.Config
	checkWaitSeconds     int
	checkTTLHours        int
	reportTTLHours       int
	cleanupIntervalMins  int
	tokenSecret          string
	rabbitmqURL          string
	amlbotBaseURL        string
	amlbotAPIKey         string
	chainalysisAPIKey    string
	objectStorageEnabled bool
	objectStorageConfig  objectStorageConfig
}

func (app *application) mount() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{env.GetString("CORS_ALLOWED_ORIGIN", "http://localhost:5174")},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(app.RateLimiterMiddleware)

	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/v1", func(r chi.Router) {
		r.Get("/health", app.healthCheckHandler)

		r.Post("/check-address", app.handlers.CheckAddress)
		r.Get("/check-address/{check_id}", app.handlers.GetCheckStatus)
		r.Get("/report/{token}", app.handlers.GetReport)

		docsURL := fmt.Sprintf("%s/swagger/doc.json", app.config.addr)
		r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL(docsURL)))
	})

	return r
}

func (app *application) run(mux http.Handler) error {
	// docs
	docs.SwaggerInfo.Version = version
	docs.SwaggerInfo.Host = "localhost:8080"
	docs.SwaggerInfo.BasePath = "/v1"
	docs.SwaggerInfo.Schemes = []string{"http"}

	srv := &http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	// graceful shutdown
	shutdown := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		app.logger.Infow("signal caught", "signal", s.String())

		shutdown <- srv.Shutdown(ctx)
	}()

	app.logger.Infow("server have started", "addr", app.config.addr, "env", app.config.env)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdown
	if err != nil {
		return err
	}

	app.logger.Infow("server has stopped", "addr", app.config.addr, "env", app.config.env)

	return nil
}
