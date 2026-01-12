// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
)

// correlationIDKeyType is the context key type for correlation IDs.
type correlationIDKeyType struct{}

var correlationIDKey = correlationIDKeyType{}

// NewCorrelationID generates a new correlation ID.
// Returns a hex-encoded random ID. On entropy failure, logs a warning and
// returns an ID with available bytes (may be less random).
func NewCorrelationID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		log.Printf(`{"level":"warn","msg":"failed to generate correlation ID",`+
			`"error":"%v"}`, err)
	}
	return hex.EncodeToString(b)
}

// WithCorrelationID adds a correlation ID to the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from context.
// Returns empty string if not found.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}
