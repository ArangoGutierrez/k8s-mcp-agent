// Copyright 2026 k8s-gpu-mcp-server contributors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// DefaultAgentHTTPPort is the default port agents listen on in HTTP mode.
const DefaultAgentHTTPPort = 8080

// AgentHTTPClient handles HTTP communication with agent pods.
type AgentHTTPClient struct {
	client      *http.Client
	retryPolicy RetryPolicy
}

// RetryPolicy defines retry behavior for failed requests.
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy returns sensible retry defaults.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   2 * time.Second,
	}
}

// NewAgentHTTPClient creates an HTTP client optimized for agent communication.
func NewAgentHTTPClient() *AgentHTTPClient {
	return &AgentHTTPClient{
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		retryPolicy: DefaultRetryPolicy(),
	}
}

// CallMCP sends an MCP request to an agent pod and returns the response.
// The endpoint should be the full URL (e.g., "http://10.0.0.5:8080").
func (c *AgentHTTPClient) CallMCP(
	ctx context.Context,
	endpoint string,
	request []byte,
) ([]byte, error) {
	url := endpoint + "/mcp"

	var lastErr error
	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			log.Printf(`{"level":"debug","msg":"retrying request",`+
				`"attempt":%d,"delay":"%s","endpoint":"%s"}`,
				attempt, delay, endpoint)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		response, err := c.doRequest(ctx, url, request)
		if err == nil {
			return response, nil
		}
		lastErr = err

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w",
		c.retryPolicy.MaxRetries+1, lastErr)
}

// doRequest performs a single HTTP request.
func (c *AgentHTTPClient) doRequest(
	ctx context.Context,
	url string,
	body []byte,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s",
			resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// calculateBackoff returns the delay for a retry attempt using exponential
// backoff. Delays are capped at MaxDelay.
func (c *AgentHTTPClient) calculateBackoff(attempt int) time.Duration {
	delay := c.retryPolicy.BaseDelay * time.Duration(1<<uint(attempt-1))
	if delay > c.retryPolicy.MaxDelay {
		delay = c.retryPolicy.MaxDelay
	}
	return delay
}
