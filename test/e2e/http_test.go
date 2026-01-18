// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_HealthzEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/healthz")
	require.NoError(t, err, "Failed to call /healthz")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "healthz should return 200")

	var body map[string]string
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err, "Failed to decode healthz response")
	assert.Equal(t, "healthy", body["status"], "healthz should return healthy status")
}

func TestHTTP_ReadyzEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/readyz")
	require.NoError(t, err, "Failed to call /readyz")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "readyz should return 200")

	var body map[string]string
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err, "Failed to decode readyz response")
	assert.Equal(t, "ready", body["status"], "readyz should return ready status")
}

func TestHTTP_MetricsEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/metrics")
	require.NoError(t, err, "Failed to call /metrics")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "metrics should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read metrics response")

	bodyStr := string(body)
	t.Logf("Metrics response length: %d bytes", len(body))

	// Verify Prometheus format
	assert.Contains(t, bodyStr, "# HELP", "Metrics should contain HELP comments")
	assert.Contains(t, bodyStr, "# TYPE", "Metrics should contain TYPE definitions")

	// Verify some expected metrics
	assert.Contains(t, bodyStr, "go_goroutines",
		"Metrics should include Go runtime metrics")
}

func TestHTTP_VersionEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/version")
	require.NoError(t, err, "Failed to call /version")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "version should return 200")

	var body map[string]string
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err, "Failed to decode version response")

	assert.Contains(t, body, "server", "version should contain server field")
	assert.Contains(t, body, "version", "version should contain version field")
	assert.Equal(t, "k8s-gpu-mcp-server", body["server"],
		"server name should be k8s-gpu-mcp-server")
}

func TestHTTP_ResponseTime(t *testing.T) {
	start := time.Now()
	resp, err := http.Get(gatewayURL + "/healthz")
	elapsed := time.Since(start)

	require.NoError(t, err, "Failed to call /healthz")
	_ = resp.Body.Close()

	t.Logf("Health endpoint response time: %v", elapsed)

	// Health endpoint should respond within 500ms
	assert.Less(t, elapsed, 500*time.Millisecond,
		"Health endpoint should respond within 500ms")
}

func TestHTTP_ContentTypeHeaders(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		expected string
	}{
		{"healthz", "/healthz", "application/json"},
		{"readyz", "/readyz", "application/json"},
		{"version", "/version", "application/json"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(gatewayURL + tc.endpoint)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			contentType := resp.Header.Get("Content-Type")
			assert.Contains(t, contentType, tc.expected,
				"%s should have %s content type", tc.endpoint, tc.expected)
		})
	}
}

func TestHTTP_MethodNotAllowed(t *testing.T) {
	// Health endpoints should only allow GET
	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		endpoint string
		method   string
	}{
		{"/healthz", http.MethodPost},
		{"/readyz", http.MethodPost},
		{"/version", http.MethodPost},
		{"/healthz", http.MethodPut},
		{"/readyz", http.MethodDelete},
	}

	for _, tc := range testCases {
		t.Run(tc.method+"_"+tc.endpoint, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, gatewayURL+tc.endpoint, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode,
				"%s %s should return 405", tc.method, tc.endpoint)
		})
	}
}
