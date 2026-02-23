package client

import (
	"net/http"
	"testing"
	"time"
)

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 60 * time.Second}
	c := &Client{}
	
	opt := WithHTTPClient(customClient)
	opt(c)
	
	if c.httpClient != customClient {
		t.Error("WithHTTPClient did not set custom HTTP client")
	}
}

func TestWithLogger(t *testing.T) {
	logger := &testLogger{}
	c := &Client{}
	
	opt := WithLogger(logger)
	opt(c)
	
	if c.logger != logger {
		t.Error("WithLogger did not set custom logger")
	}
}

func TestWithRetryMax(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"positive value", 5, 5},
		{"zero value", 0, 0},
		{"negative value", -1, 0}, // should not change from default 0
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{retryMax: 0}
			opt := WithRetryMax(tt.input)
			opt(c)
			
			if tt.input >= 0 && c.retryMax != tt.expected {
				t.Errorf("WithRetryMax(%d): got %d, want %d", tt.input, c.retryMax, tt.expected)
			}
		})
	}
}

func TestWithRetryWait(t *testing.T) {
	tests := []struct {
		name        string
		min         time.Duration
		max         time.Duration
		expectMin   time.Duration
		expectMax   time.Duration
	}{
		{"valid range", 1 * time.Second, 5 * time.Second, 1 * time.Second, 5 * time.Second},
		{"equal values", 2 * time.Second, 2 * time.Second, 2 * time.Second, 2 * time.Second},
		{"zero min", 0, 5 * time.Second, 0, 0}, // min not set if zero
		{"max less than min", 5 * time.Second, 2 * time.Second, 5 * time.Second, 0}, // max not set if less than min
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{}
			opt := WithRetryWait(tt.min, tt.max)
			opt(c)
			
			if tt.min > 0 && c.retryWaitMin != tt.expectMin {
				t.Errorf("retryWaitMin: got %v, want %v", c.retryWaitMin, tt.expectMin)
			}
			if tt.max > 0 && tt.max >= tt.min && c.retryWaitMax != tt.expectMax {
				t.Errorf("retryWaitMax: got %v, want %v", c.retryWaitMax, tt.expectMax)
			}
		})
	}
}

func TestWithUserAgent(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldSet bool
	}{
		{"non-empty string", "custom-agent/1.0", true},
		{"empty string", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{userAgent: "default"}
			opt := WithUserAgent(tt.input)
			opt(c)
			
			if tt.shouldSet {
				if c.userAgent != tt.input {
					t.Errorf("WithUserAgent(%q): got %q, want %q", tt.input, c.userAgent, tt.input)
				}
			} else {
				if c.userAgent != "default" {
					t.Error("WithUserAgent should not set empty string")
				}
			}
		})
	}
}

// testLogger implements Logger interface for testing
type testLogger struct {
	debugCalled bool
	infoCalled  bool
	errorCalled bool
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	l.debugCalled = true
}

func (l *testLogger) Infof(format string, args ...interface{}) {
	l.infoCalled = true
}

func (l *testLogger) Errorf(format string, args ...interface{}) {
	l.errorCalled = true
}

//Personal.AI order the ending
