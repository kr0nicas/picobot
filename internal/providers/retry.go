package providers

import (
	"context"
	"math"
	"net/http"
	"time"
)

const (
	maxRetries    = 3
	baseDelay     = 500 * time.Millisecond
	maxDelay      = 10 * time.Second
)

// retryableStatusCode returns true for HTTP status codes that warrant a retry.
func retryableStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError,  // 500
		http.StatusBadGateway,           // 502
		http.StatusServiceUnavailable,   // 503
		http.StatusGatewayTimeout:       // 504
		return true
	}
	return false
}

// backoffDelay returns the delay for the given attempt using exponential backoff.
func backoffDelay(attempt int) time.Duration {
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// doWithRetry executes an HTTP request with retries for transient errors.
// It returns the response only when it's a non-retryable status or retries are exhausted.
func doWithRetry(ctx context.Context, client *http.Client, buildReq func() (*http.Request, error)) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var req *http.Request
		req, err = buildReq()
		if err != nil {
			return nil, err
		}

		resp, err = client.Do(req)
		if err != nil {
			// Network errors are retryable
			continue
		}

		if !retryableStatusCode(resp.StatusCode) {
			return resp, nil
		}

		// Close body before retry to avoid leaking connections
		resp.Body.Close()
	}

	// Return the last response or error
	if err != nil {
		return nil, err
	}
	return resp, nil
}
