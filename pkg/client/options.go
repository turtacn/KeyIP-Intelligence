package client

import (
	"net/http"
	"time"
)

// Option is a functional option for configuring the Client
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithLogger sets a custom logger
func WithLogger(logger Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithRetryMax sets the maximum number of retries
func WithRetryMax(retryMax int) Option {
	return func(c *Client) {
		if retryMax >= 0 {
			c.retryMax = retryMax
		}
	}
}

// WithRetryWait sets the minimum and maximum retry wait durations
// Both min and max must be positive, and max must be >= min for values to be set
func WithRetryWait(min, max time.Duration) Option {
	return func(c *Client) {
		if min > 0 {
			c.retryWaitMin = min
			// Only set max if min is valid and max >= min
			if max >= min {
				c.retryWaitMax = max
			}
		}
	}
}

// WithUserAgent sets a custom User-Agent string
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if userAgent != "" {
			c.userAgent = userAgent
		}
	}
}

//Personal.AI order the ending
