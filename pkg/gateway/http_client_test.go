// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentHTTPClient_CallMCP_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/mcp", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(
				[]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`))
		}))
	defer server.Close()

	client := NewAgentHTTPClient()
	resp, err := client.CallMCP(context.Background(), server.URL, []byte(`{}`))

	require.NoError(t, err)
	assert.Contains(t, string(resp), "jsonrpc")
}

func TestAgentHTTPClient_CallMCP_RetryOnFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
	defer server.Close()

	client := NewAgentHTTPClient()
	client.retryPolicy.BaseDelay = 10 * time.Millisecond // Speed up test

	resp, err := client.CallMCP(context.Background(), server.URL, []byte(`{}`))

	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
	assert.Contains(t, string(resp), "success")
}

func TestAgentHTTPClient_CallMCP_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	client := NewAgentHTTPClient()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.CallMCP(ctx, server.URL, []byte(`{}`))

	assert.Error(t, err)
}

func TestAgentHTTPClient_CallMCP_AllRetriesFail(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	defer server.Close()

	client := NewAgentHTTPClient()
	client.retryPolicy.MaxRetries = 2
	client.retryPolicy.BaseDelay = 1 * time.Millisecond

	_, err := client.CallMCP(context.Background(), server.URL, []byte(`{}`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	assert.Equal(t, 3, policy.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, policy.BaseDelay)
	assert.Equal(t, 2*time.Second, policy.MaxDelay)
}

func TestAgentHTTPClient_calculateBackoff(t *testing.T) {
	client := NewAgentHTTPClient()
	client.retryPolicy.BaseDelay = 100 * time.Millisecond
	client.retryPolicy.MaxDelay = 1 * time.Second

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond}, // 100ms * 2^0
		{2, 200 * time.Millisecond}, // 100ms * 2^1
		{3, 400 * time.Millisecond}, // 100ms * 2^2
		{4, 800 * time.Millisecond}, // 100ms * 2^3
		{5, 1 * time.Second},        // capped at MaxDelay
	}

	for _, tt := range tests {
		delay := client.calculateBackoff(tt.attempt)
		assert.Equal(t, tt.expected, delay, "attempt %d", tt.attempt)
	}
}

func TestNewAgentHTTPClient(t *testing.T) {
	client := NewAgentHTTPClient()

	assert.NotNil(t, client.client)
	assert.Equal(t, 60*time.Second, client.client.Timeout)
	assert.Equal(t, DefaultRetryPolicy(), client.retryPolicy)
}
