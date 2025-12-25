package main

import (
	"context"
	"expvar"
	"runtime"
	"time"

	"github.com/Beka01247/bitpanda-aml/internal/domain"
	"github.com/Beka01247/bitpanda-aml/internal/env"
	"github.com/Beka01247/bitpanda-aml/internal/infrastructure/billing"
	"github.com/Beka01247/bitpanda-aml/internal/infrastructure/providers"
	"github.com/Beka01247/bitpanda-aml/internal/infrastructure/rabbitmq"
	"github.com/Beka01247/bitpanda-aml/internal/infrastructure/repositories"
	"github.com/Beka01247/bitpanda-aml/internal/infrastructure/storage"
	"github.com/Beka01247/bitpanda-aml/internal/infrastructure/token"
	"github.com/Beka01247/bitpanda-aml/internal/ratelimiter"
	httpTransport "github.com/Beka01247/bitpanda-aml/internal/transport/http"
	"github.com/Beka01247/bitpanda-aml/internal/workers"
	"go.uber.org/zap"

	app "github.com/Beka01247/bitpanda-aml/internal/application"
)

const version = "0.0.1"

//	@title			Bitpanda AML
//	@description	API for Bitpanda AML
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	http://www.swagger.io/support
//	@contact.email	support@swagger.io

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

// @host						localhost:8080
// @schemes					http
// @BasePath					/v1
//
// @securityDefinitions.apiKey	ApiKeyAuth
// @in							header
// @name						Authorization
// @description
func main() {
	cfg := config{
		addr:        env.GetString("ADDR", ":8080"),
		apiURL:      env.GetString("EXTERNAL_URL", "http://localhost:8080"),
		frontendURL: env.GetString("FRONTEND_URL", "localhost:3000"),
		env:         env.GetString("ENV", "development"),
		rateLimiter: ratelimiter.Config{
			RequestsPerTimeFrame: env.GetInt("RATELIMITER_REQUESTS_COUNT", 20),
			TimeFrame:            time.Second * 5,
			Enabled:              env.GetBool("RATE_LIMITER_ENABLED", true),
		},
		checkWaitSeconds:     env.GetInt("CHECK_WAIT_SECONDS", 20),
		checkTTLHours:        env.GetInt("CHECK_TTL_HOURS", 24),
		reportTTLHours:       env.GetInt("REPORT_TTL_HOURS", 24),
		cleanupIntervalMins:  env.GetInt("CLEANUP_INTERVAL_MINUTES", 10),
		tokenSecret:          env.GetString("TOKEN_SECRET", "change-me-in-production"),
		rabbitmqURL:          env.GetString("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		amlbotBaseURL:        env.GetString("AMLBOT_BASE_URL", ""),
		amlbotAPIKey:         env.GetString("AMLBOT_API_KEY", ""),
		chainalysisAPIKey:    env.GetString("CHAINALYSIS_API_KEY", ""),
		objectStorageEnabled: env.GetBool("OBJECT_STORAGE_ENABLED", false),
		objectStorageConfig: objectStorageConfig{
			endpoint:  env.GetString("OBJECT_STORAGE_ENDPOINT", "localhost:9000"),
			publicURL: env.GetString("OBJECT_STORAGE_PUBLIC_URL", "http://localhost:9000"),
			accessKey: env.GetString("OBJECT_STORAGE_ACCESS_KEY", "minioadmin"),
			secretKey: env.GetString("OBJECT_STORAGE_SECRET_KEY", "minioadmin"),
			bucket:    env.GetString("OBJECT_STORAGE_BUCKET", "reports"),
			useSSL:    env.GetBool("OBJECT_STORAGE_USE_SSL", false),
		},
	}

	// logger
	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	logger.Infow("starting application", "version", version, "env", cfg.env)

	// initialize infrastructure
	ctx := context.Background()

	// asset registry
	assetRegistry := domain.NewDefaultAssetRegistry()
	logger.Info("asset registry initialized")

	// rabbitMQ
	messageBus, err := rabbitmq.NewRabbitMQBus(cfg.rabbitmqURL, logger)
	if err != nil {
		logger.Fatalw("failed to initialize rabbitmq", "error", err)
	}
	defer messageBus.Close()

	// AML provider
	var amlProvider domain.AMLProvider
	if cfg.amlbotAPIKey != "" && cfg.amlbotBaseURL != "" {
		amlProvider = providers.NewAMLBotProvider(cfg.amlbotBaseURL, cfg.amlbotAPIKey, logger)
		logger.Infow("using AMLBot provider", "base_url", cfg.amlbotBaseURL)
	} else {
		amlProvider = providers.NewMockAMLProvider(logger)
		logger.Warn("using mock AML provider (no AMLBot credentials)")
	}

	// sanctions provider
	sanctionsProvider := providers.NewChainalysisProvider(cfg.chainalysisAPIKey, logger)
	if cfg.chainalysisAPIKey == "" {
		logger.Warn("chainalysis api key not set, sanctions checks will return empty results")
	} else {
		logger.Info("chainalysis provider initialized")
	}

	// repository
	checkRepository := repositories.NewMemoryCheckRepository(logger)
	checkRepository.StartCleanupLoop(ctx, time.Duration(cfg.cleanupIntervalMins)*time.Minute)
	logger.Info("check repository initialized")

	// report storage
	var reportStorage domain.ReportStorage
	if cfg.objectStorageEnabled {
		minioStorage, err := storage.NewMinIOStorage(
			cfg.objectStorageConfig.endpoint,
			cfg.objectStorageConfig.accessKey,
			cfg.objectStorageConfig.secretKey,
			cfg.objectStorageConfig.bucket,
			cfg.objectStorageConfig.useSSL,
			cfg.objectStorageConfig.publicURL,
			logger,
		)
		if err != nil {
			logger.Fatalw("failed to initialize minio storage", "error", err)
		}
		minioStorage.StartCleanupLoop(ctx, time.Duration(cfg.cleanupIntervalMins)*time.Minute)
		reportStorage = minioStorage
	} else {
		localStorage, err := storage.NewLocalStorage("", logger)
		if err != nil {
			logger.Fatalw("failed to initialize local storage", "error", err)
		}
		localStorage.StartCleanupLoop(ctx, time.Duration(cfg.cleanupIntervalMins)*time.Minute)
		reportStorage = localStorage
	}

	// token provider
	tokenProvider := token.NewHMACToken(cfg.tokenSecret)

	// billing hook
	billingHook := billing.NewNoopBillingHook(logger)

	checkTTL := time.Duration(cfg.checkTTLHours) * time.Hour
	reportTTL := time.Duration(cfg.reportTTLHours) * time.Hour

	checkAddressUseCase := app.NewCheckAddressUseCase(assetRegistry, checkRepository, messageBus, checkTTL, logger)
	getStatusUseCase := app.NewGetCheckStatusUseCase(checkRepository, logger)
	processAMLCheckUseCase := app.NewProcessAMLCheckUseCase(amlProvider, sanctionsProvider, checkRepository, messageBus, logger)
	generateReportUseCase := app.NewGenerateReportUseCase(checkRepository, reportStorage, messageBus, billingHook, reportTTL, logger)
	handleCheckFailedUseCase := app.NewHandleCheckFailedUseCase(checkRepository, logger)

	// workers
	amlWorker := workers.NewAMLWorker(processAMLCheckUseCase, messageBus, logger)
	if err := amlWorker.Start(); err != nil {
		logger.Fatalw("failed to start aml worker", "error", err)
	}
	defer amlWorker.Stop()

	reportWorker := workers.NewReportWorker(generateReportUseCase, handleCheckFailedUseCase, messageBus, logger)
	if err := reportWorker.Start(); err != nil {
		logger.Fatalw("failed to start report worker", "error", err)
	}
	defer reportWorker.Stop()

	// HTTP handlers
	handlers := httpTransport.NewHandlers(
		checkAddressUseCase,
		getStatusUseCase,
		reportStorage,
		tokenProvider,
		cfg.checkWaitSeconds,
		cfg.apiURL,
		logger,
	)

	// rate limiter
	rateLimiter := ratelimiter.NewFixedWindowLimiter(
		cfg.rateLimiter.RequestsPerTimeFrame,
		cfg.rateLimiter.TimeFrame,
	)

	apiApp := &application{
		config:      cfg,
		logger:      logger,
		rateLimiter: rateLimiter,
		handlers:    handlers,
	}

	// metrics
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	mux := apiApp.mount()

	logger.Fatal(apiApp.run(mux))
}
