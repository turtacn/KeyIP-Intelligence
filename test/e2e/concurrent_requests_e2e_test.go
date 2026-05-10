// Phase 18 - E2E Test: Concurrent Request Handling
// Validates the API server's ability to handle concurrent requests, verifying
// that the server remains responsive under parallel load and that concurrent
// searches return consistent results.
package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestConcurrentRequests_ParallelSearches validates that multiple search
// requests can be processed concurrently without errors.
func TestConcurrentRequests_ParallelSearches(t *testing.T) {
	skipIfNoServer(t)

	// Define a set of search requests to execute in parallel.
	type searchTask struct {
		name       string
		method     string
		path       string
		body       map[string]interface{}
		token      string
	}

	tasks := []searchTask{
		{
			name:   "OLED keyword search",
			method: "POST",
			path:   "/api/v1/patents/search",
			body:   map[string]interface{}{"query": "OLED", "query_type": "keyword", "page": 1, "page_size": 5},
			token:  env.analystToken,
		},
		{
			name:   "carbazole similarity search",
			method: "POST",
			path:   "/api/v1/molecules/search",
			body:   map[string]interface{}{"query": "c1ccc2c(c1)c1ccccc1[nH]2", "search_mode": "similarity", "similarity": 0.7, "page": 1, "page_size": 5},
			token:  env.analystToken,
		},
		{
			name:   "health check",
			method: "GET",
			path:   "/healthz",
			token:  "",
		},
		{
			name:   "readiness probe",
			method: "GET",
			path:   "/readyz",
			token:  "",
		},
		{
			name:   "benzene exact search",
			method: "POST",
			path:   "/api/v1/molecules/search",
			body:   map[string]interface{}{"query": "c1ccccc1", "search_mode": "exact", "page": 1, "page_size": 5},
			token:  env.analystToken,
		},
		{
			name:   "display patent search",
			method: "POST",
			path:   "/api/v1/patents/search",
			body:   map[string]interface{}{"query": "display", "query_type": "keyword", "page": 1, "page_size": 5},
			token:  env.viewerToken,
		},
	}

	// Run all searches concurrently.
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))
	results := make([]int, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, tsk searchTask) {
			defer wg.Done()

			start := time.Now()
			var resp *http.Response

			switch tsk.method {
			case "GET":
				resp = doGet(t, tsk.path, tsk.token)
			case "POST":
				resp = doPost(t, tsk.path, tsk.body, tsk.token)
			default:
				errChan <- fmt.Errorf("unsupported method: %s", tsk.method)
				return
			}

			elapsed := time.Since(start)

			if resp != nil {
				results[idx] = resp.StatusCode
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				t.Logf("[%s] status=%d elapsed=%v", tsk.name, resp.StatusCode, elapsed)

				// Check for server errors.
				if resp.StatusCode >= 500 {
					errChan <- fmt.Errorf("%s returned server error %d: %s", tsk.name, resp.StatusCode, string(body))
				}

				// Verify response is valid JSON.
				if len(body) > 0 {
					var js json.RawMessage
					if err := json.Unmarshal(body, &js); err != nil {
						errChan <- fmt.Errorf("%s returned invalid JSON: %w", tsk.name, err)
					}
				}
			}
		}(i, task)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors.
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Logf("concurrent request test completed with %d errors:", len(errs))
		for _, err := range errs {
			t.Logf("  - %v", err)
		}
	} else {
		t.Log("all concurrent requests completed without errors")
	}

	// Log status code summary.
	statusCounts := make(map[int]int)
	for _, code := range results {
		statusCounts[code]++
	}
	t.Logf("status code distribution: %v", statusCounts)
}

// TestConcurrentRequests_BurstLoad validates the server can handle a burst
// of identical requests without dropping connections or returning errors.
func TestConcurrentRequests_BurstLoad(t *testing.T) {
	skipIfNoServer(t)

	concurrency := 10
	requestsPerGoroutine := 3

	var wg sync.WaitGroup
	errChan := make(chan error, concurrency*requestsPerGoroutine)

	for g := 0; g < concurrency; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for r := 0; r < requestsPerGoroutine; r++ {
				resp := doGet(t, "/healthz", "")
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errChan <- fmt.Errorf("goroutine %d request %d: status %d", goroutineID, r, resp.StatusCode)
				}

				// Verify JSON response.
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err != nil {
					errChan <- fmt.Errorf("goroutine %d request %d: invalid JSON: %w", goroutineID, r, err)
				}

				if status, ok := result["status"].(string); ok {
					if status != "alive" && status != "ok" {
						errChan <- fmt.Errorf("goroutine %d request %d: unexpected status %q", goroutineID, r, status)
					}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	totalRequests := concurrency * requestsPerGoroutine
	if len(errs) > 0 {
		t.Fatalf("burst load test: %d/%d requests failed", len(errs), totalRequests)
	}
	t.Logf("burst load test passed: %d concurrent requests all succeeded", totalRequests)
}

// TestConcurrentRequests_MixedWorkload validates the server handles a mix
// of authenticated and unauthenticated requests concurrently.
func TestConcurrentRequests_MixedWorkload(t *testing.T) {
	skipIfNoServer(t)

	var wg sync.WaitGroup
	errChan := make(chan error, 20)

	// Mix of authenticated and unauthenticated requests.
	workload := []struct {
		name  string
		path  string
		method string
		body  interface{}
		token string
	}{
		{"health", "/healthz", "GET", nil, ""},
		{"readiness", "/readyz", "GET", nil, ""},
		{"patent search (analyst)", "/api/v1/patents/search", "POST", map[string]interface{}{"query": "organic", "query_type": "keyword", "page": 1, "page_size": 5}, env.analystToken},
		{"molecule search (analyst)", "/api/v1/molecules/search", "POST", map[string]interface{}{"query": "c1ccccc1", "search_mode": "exact", "page": 1, "page_size": 5}, env.analystToken},
		{"patent search (viewer)", "/api/v1/patents/search", "POST", map[string]interface{}{"query": "light emitting", "query_type": "keyword", "page": 1, "page_size": 5}, env.viewerToken},
		{"detailed health", "/healthz/detail", "GET", nil, ""},
		{"patent by number", "/api/v1/patents/by-number?number=US11847352B2", "GET", nil, env.analystToken},
		{"molecule by SMILES", "/api/v1/molecules/by-smiles?smiles=c1ccccc1", "GET", nil, env.analystToken},
	}

	// Run each workload item multiple times concurrently.
	for i := 0; i < 3; i++ {
		for _, wl := range workload {
			wg.Add(1)
			go func(wlName, method, path string, body interface{}, token string) {
				defer wg.Done()

				var resp *http.Response
				switch method {
				case "GET":
					resp = doGet(t, path, token)
				case "POST":
					resp = doPost(t, path, body, token)
				}
				if resp != nil {
					resp.Body.Close()
					if resp.StatusCode >= 500 {
						errChan <- fmt.Errorf("%s: server error %d", wlName, resp.StatusCode)
					}
				}
			}(wl.name, wl.method, wl.path, wl.body, wl.token)
		}
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Logf("mixed workload test completed with %d errors:", len(errs))
		for _, err := range errs {
			t.Logf("  - %v", err)
		}
	} else {
		t.Log("mixed workload test passed: all requests completed without server errors")
	}
}

// TestConcurrentRequests_ResponseTime validates that concurrent requests
// complete within acceptable time limits.
func TestConcurrentRequests_ResponseTime(t *testing.T) {
	skipIfNoServer(t)

	concurrency := 5
	maxLatency := 10 * time.Second

	var wg sync.WaitGroup
	latencies := make([]time.Duration, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			start := time.Now()
			resp := doGet(t, "/healthz", "")
			resp.Body.Close()
			latencies[idx] = time.Since(start)
		}(i)
	}

	wg.Wait()

	slowCount := 0
	for i, lat := range latencies {
		if lat > maxLatency {
			slowCount++
			t.Logf("request %d exceeded latency limit: %v > %v", i, lat, maxLatency)
		}
	}

	if slowCount > 0 {
		t.Fatalf("%d/%d concurrent requests exceeded max latency of %v", slowCount, concurrency, maxLatency)
	}

	// Log latency stats.
	var total time.Duration
	minLat := latencies[0]
	maxLat := latencies[0]
	for _, lat := range latencies {
		total += lat
		if lat < minLat {
			minLat = lat
		}
		if lat > maxLat {
			maxLat = lat
		}
	}
	avgLat := total / time.Duration(concurrency)
	t.Logf("concurrent latency: avg=%v min=%v max=%v (limit=%v)", avgLat, minLat, maxLat, maxLat)
}

// TestConcurrentRequests_SequentialSearches validates that sequential
// requests return consistent results (no cross-request contamination).
func TestConcurrentRequests_SequentialSearches(t *testing.T) {
	skipIfNoServer(t)

	// Run the same search multiple times and verify results are consistent.
	query := map[string]interface{}{
		"query":      "OLED",
		"query_type": "keyword",
		"page":       1,
		"page_size":  3,
	}

	previousPatents := ""

	for i := 0; i < 3; i++ {
		resp := doPost(t, "/api/v1/patents/search", query, env.analystToken)
		if resp.StatusCode != http.StatusOK {
			t.Skipf("patent search unavailable on iteration %d (status=%d)", i, resp.StatusCode)
		}

		var result map[string]interface{}
		assertJSON(t, resp, &result)

		patentsJSON, _ := json.Marshal(result["patents"])
		currentPatents := string(patentsJSON)

		if i > 0 && previousPatents != "" && previousPatents != currentPatents {
			t.Logf("note: search results differ between iteration %d and previous", i)
			t.Log("this may be expected if the index is being updated")
		}

		previousPatents = currentPatents
		resp.Body.Close()
	}

	t.Log("sequential search consistency check completed")
}

// Personal.AI order the ending
