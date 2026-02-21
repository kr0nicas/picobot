package providers

import (
	"context"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

const (
	maxRetries    = 3
	baseDelay     = 1 * time.Second
	maxDelay      = 60 * time.Second
	rateLimitBase = 5 * time.Second // longer base delay for 429
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

// retryAfterDelay parses the Retry-After header if present and returns the delay.
// Returns 0 if not present or unparseable.
func retryAfterDelay(resp *http.Response) time.Duration {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}
	// Try as seconds first
	if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	// Try as HTTP date
	if t, err := http.ParseTime(val); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

// doWithRetry executes an HTTP request with retries for transient errors.
// It respects the Retry-After header for 429 responses.
func doWithRetry(ctx context.Context, client *http.Client, buildReq func() (*http.Request, error)) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(attempt - 1)
			// For 429, use longer base delay or Retry-After header
			if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
				delay = time.Duration(float64(rateLimitBase) * math.Pow(2, float64(attempt-1)))
				if ra := retryAfterDelay(resp); ra > 0 && ra <= 60*time.Second {
					delay = ra
				}
				if delay > maxDelay {
					delay = maxDelay
				}
			}
			log.Printf("provider: retrying request (attempt %d/%d, waiting %v)", attempt, maxRetries, delay)
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
