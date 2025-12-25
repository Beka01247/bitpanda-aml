# Bitpanda AML - Cryptocurrency Address Risk Assessment

A Go backend service for cryptocurrency address AML (Anti-Money Laundering) checks with asynchronous processing, sanctions screening, and PDF report generation.

## Features

- **Multi-Currency Support**: BTC, ETH, USDT with extensible asset registry
- **AML Provider Integration**: AMLBot integration with mock fallback
- **Sanctions Screening**: Chainalysis API integration for OFAC sanctions checks
- **Event-Driven Architecture**: RabbitMQ-based async processing pipeline
- **PDF Report Generation**: Valid PDF reports with risk assessment and sanctions data
- **Temporary Storage**: MinIO (S3-compatible) or local filesystem with automatic cleanup
- **Rate Limiting**: IP-based rate limiting with configurable thresholds

## Quick Start

```bash
# Copy environment variables
cp .env.example .env

# Start services
docker-compose up --build
```

Access Swagger documentation at: http://localhost:8080/v1/swagger/index.html

## Configuration

All configuration is done via environment variables. See [.env.example](.env.example) for available options and defaults.

## API Documentation

Full API documentation with request/response examples is available in Swagger UI at `/v1/swagger/index.html` when the server is running.

## Architecture

Clean architecture with event-driven async processing:

1. **Request** → API validates and publishes `aml.check.requested`
2. **AML Worker** → Calls providers and publishes `aml.check.completed`
3. **Report Worker** → Generates PDF and publishes `aml.report.ready`
4. **Response** → Risk score, sanctions data, and signed PDF URL

## Testing

The codebase includes unit tests for domain logic, infrastructure components, and core business rules.

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```
