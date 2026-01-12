// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCorrelationID(t *testing.T) {
	id1 := NewCorrelationID()
	id2 := NewCorrelationID()

	// Should generate 16 character hex strings (8 bytes = 16 hex chars)
	assert.Len(t, id1, 16)
	assert.Len(t, id2, 16)

	// Should be unique
	assert.NotEqual(t, id1, id2)
}

func TestWithCorrelationID(t *testing.T) {
	ctx := context.Background()
	id := "test-correlation-id"

	ctx = WithCorrelationID(ctx, id)

	// Should be able to retrieve the ID
	retrieved := CorrelationIDFromContext(ctx)
	assert.Equal(t, id, retrieved)
}

func TestCorrelationIDFromContext_NotFound(t *testing.T) {
	ctx := context.Background()

	// Should return empty string if not found
	retrieved := CorrelationIDFromContext(ctx)
	assert.Equal(t, "", retrieved)
}

func TestCorrelationID_Propagation(t *testing.T) {
	ctx := context.Background()
	id := NewCorrelationID()

	// Set ID in parent context
	ctx = WithCorrelationID(ctx, id)

	// Create child context (simulating passing to goroutine)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// ID should propagate to child
	retrieved := CorrelationIDFromContext(childCtx)
	assert.Equal(t, id, retrieved)
}
