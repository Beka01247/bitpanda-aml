package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type HMACToken struct {
	secret []byte
}

func NewHMACToken(secret string) *HMACToken {
	return &HMACToken{
		secret: []byte(secret),
	}
}

func (h *HMACToken) Sign(key string, expires time.Duration) string {
	expiresAt := time.Now().UTC().Add(expires).Unix()
	payload := fmt.Sprintf("%s:%d", key, expiresAt)

	// base64 encode the payload to avoid issues with special chars
	encodedPayload := base64.URLEncoding.EncodeToString([]byte(payload))

	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(encodedPayload))
	signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", encodedPayload, signature)
}

func (h *HMACToken) Verify(token string) (string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token format")
	}

	encodedPayload := parts[0]
	providedSig := parts[1]

	// verify signature
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(encodedPayload))
	expectedSig := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid token signature")
	}

	// decode payload
	payloadBytes, err := base64.URLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", fmt.Errorf("invalid token encoding: %w", err)
	}

	payload := string(payloadBytes)

	// parse payload
	payloadParts := strings.SplitN(payload, ":", 2)
	if len(payloadParts) != 2 {
		return "", fmt.Errorf("invalid token payload")
	}

	key := payloadParts[0]
	expiresAt, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid expiration time")
	}

	// check expiration
	if time.Now().UTC().Unix() > expiresAt {
		return "", fmt.Errorf("token expired")
	}

	return key, nil
}
