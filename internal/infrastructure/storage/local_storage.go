package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type LocalStorage struct {
	basePath string
	metadata map[string]*fileMetadata
	mu       sync.RWMutex
	logger   *zap.SugaredLogger
}

type fileMetadata struct {
	Key       string    `json:"key"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewLocalStorage(basePath string, logger *zap.SugaredLogger) (*LocalStorage, error) {
	if basePath == "" {
		basePath = filepath.Join(os.TempDir(), "bitpanda-aml", "reports")
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	storage := &LocalStorage{
		basePath: basePath,
		metadata: make(map[string]*fileMetadata),
		logger:   logger,
	}

	// load metadata from disk if exists
	storage.loadMetadata()

	logger.Infow("local storage initialized", "path", basePath)

	return storage, nil
}

func (s *LocalStorage) Put(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.basePath, key)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	expiresAt := time.Now().UTC().Add(ttl)
	s.metadata[key] = &fileMetadata{
		Key:       key,
		ExpiresAt: expiresAt,
	}

	s.saveMetadata()

	s.logger.Debugw("report stored", "key", key, "size", len(data), "expires_at", expiresAt)

	return nil
}

func (s *LocalStorage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	meta, exists := s.metadata[key]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("report not found")
	}

	if time.Now().UTC().After(meta.ExpiresAt) {
		return nil, fmt.Errorf("report expired")
	}

	filePath := filepath.Join(s.basePath, key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

func (s *LocalStorage) PresignGet(ctx context.Context, key string, expires time.Duration) (string, error) {
	// for local storage, we return empty string to indicate streaming should be used
	return "", nil
}

func (s *LocalStorage) CleanupExpired(ctx context.Context, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for key, meta := range s.metadata {
		if now.After(meta.ExpiresAt) {
			filePath := filepath.Join(s.basePath, key)
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				s.logger.Warnw("failed to remove expired file", "key", key, "error", err)
			}
			delete(s.metadata, key)
			count++
		}
	}

	if count > 0 {
		s.saveMetadata()
		s.logger.Infow("expired reports cleaned", "count", count)
	}

	return count, nil
}

// starts a background cleanup loop
func (s *LocalStorage) StartCleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("storage cleanup loop stopped")
				return
			case <-ticker.C:
				_, err := s.CleanupExpired(ctx, time.Now().UTC())
				if err != nil {
					s.logger.Errorw("storage cleanup failed", "error", err)
				}
			}
		}
	}()
}

func (s *LocalStorage) loadMetadata() {
	metaPath := filepath.Join(s.basePath, ".metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return // metadata file doesn't exist yet
	}

	var meta map[string]*fileMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		s.logger.Warnw("failed to load metadata", "error", err)
		return
	}

	s.metadata = meta
}

func (s *LocalStorage) saveMetadata() {
	metaPath := filepath.Join(s.basePath, ".metadata.json")
	data, err := json.Marshal(s.metadata)
	if err != nil {
		s.logger.Errorw("failed to marshal metadata", "error", err)
		return
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		s.logger.Errorw("failed to write metadata", "error", err)
	}
}
