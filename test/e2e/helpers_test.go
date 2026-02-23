// Phase 18 - E2E Test Helpers
// Common helper functions for E2E tests including HTTP request builders,
// response assertions, data constructors, and polling utilities.
package e2e_test

import (
"bytes"
"encoding/json"
"fmt"
"io"
"net/http"
"strings"
"testing"
"time"
)

// doGet sends a GET request to the specified path.
func doGet(t *testing.T, path string, token string) *http.Response {
t.Helper()
req, err := http.NewRequest("GET", env.baseURL+path, nil)
if err != nil {
t.Fatalf("create GET request: %v", err)
}

req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Accept", "application/json")
req.Header.Set("X-E2E-Test", "true")

resp, err := env.httpClient.Do(req)
if err != nil {
t.Fatalf("execute GET request: %v", err)
}

t.Logf("GET %s -> %d", path, resp.StatusCode)
return resp
}

// doPost sends a POST request with JSON body.
func doPost(t *testing.T, path string, body interface{}, token string) *http.Response {
t.Helper()

var bodyReader io.Reader
if body != nil {
data, err := json.Marshal(body)
if err != nil {
t.Fatalf("marshal request body: %v", err)
}
bodyReader = bytes.NewReader(data)
}

req, err := http.NewRequest("POST", env.baseURL+path, bodyReader)
if err != nil {
t.Fatalf("create POST request: %v", err)
}

req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json")
req.Header.Set("X-E2E-Test", "true")

resp, err := env.httpClient.Do(req)
if err != nil {
t.Fatalf("execute POST request: %v", err)
}

t.Logf("POST %s -> %d", path, resp.StatusCode)
return resp
}

// doPut sends a PUT request with JSON body.
func doPut(t *testing.T, path string, body interface{}, token string) *http.Response {
t.Helper()

var bodyReader io.Reader
if body != nil {
data, err := json.Marshal(body)
if err != nil {
t.Fatalf("marshal request body: %v", err)
}
bodyReader = bytes.NewReader(data)
}

req, err := http.NewRequest("PUT", env.baseURL+path, bodyReader)
if err != nil {
t.Fatalf("create PUT request: %v", err)
}

req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json")
req.Header.Set("X-E2E-Test", "true")

resp, err := env.httpClient.Do(req)
if err != nil {
t.Fatalf("execute PUT request: %v", err)
}

t.Logf("PUT %s -> %d", path, resp.StatusCode)
return resp
}

// doDelete sends a DELETE request.
func doDelete(t *testing.T, path string, token string) *http.Response {
t.Helper()
req, err := http.NewRequest("DELETE", env.baseURL+path, nil)
if err != nil {
t.Fatalf("create DELETE request: %v", err)
}

req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("X-E2E-Test", "true")

resp, err := env.httpClient.Do(req)
if err != nil {
t.Fatalf("execute DELETE request: %v", err)
}

t.Logf("DELETE %s -> %d", path, resp.StatusCode)
return resp
}

// assertStatus asserts the HTTP status code.
func assertStatus(t *testing.T, resp *http.Response, expected int) {
t.Helper()
if resp.StatusCode != expected {
body, _ := io.ReadAll(resp.Body)
t.Fatalf("expected status %d, got %d; body: %s", expected, resp.StatusCode, string(body))
}
}

// assertJSON reads and unmarshals the response body.
func assertJSON(t *testing.T, resp *http.Response, target interface{}) {
t.Helper()
body, err := io.ReadAll(resp.Body)
if err != nil {
t.Fatalf("read response body: %v", err)
}
defer resp.Body.Close()

if err := json.Unmarshal(body, target); err != nil {
t.Fatalf("unmarshal response: %v; body: %s", err, string(body))
}
}

// assertFieldExists checks that a JSON field exists.
func assertFieldExists(t *testing.T, data map[string]interface{}, field string) {
t.Helper()
if _, ok := data[field]; !ok {
t.Fatalf("expected field %q not found in response", field)
}
}

// randomSuffix generates a random suffix for test data uniqueness.
func randomSuffix() string {
return fmt.Sprintf("test-%d", time.Now().UnixNano())
}

// buildMoleculeRequest constructs a molecule creation request.
func buildMoleculeRequest(smiles, name string) map[string]interface{} {
return map[string]interface{}{
"smiles": smiles,
"name":   name,
}
}

// buildPatentRequest constructs a patent creation request.
func buildPatentRequest(number, title, assignee string) map[string]interface{} {
return map[string]interface{}{
"patent_number": number,
"title":         title,
"assignee":      assignee,
}
}

// buildPortfolioRequest constructs a portfolio creation request.
func buildPortfolioRequest(name string, patentIDs []string) map[string]interface{} {
return map[string]interface{}{
"name":       name,
"patent_ids": patentIDs,
}
}

// waitForCondition polls until condition returns true or timeout.
func waitForCondition(t *testing.T, description string, timeout time.Duration, 
interval time.Duration, condition func() bool) {
t.Helper()

deadline := time.Now().Add(timeout)
ticker := time.NewTicker(interval)
defer ticker.Stop()

for {
if condition() {
return
}

if time.Now().After(deadline) {
t.Fatalf("condition %q not met within %v", description, timeout)
}

<-ticker.C
}
}

// waitForAsyncResult polls an async task until completion.
func waitForAsyncResult(t *testing.T, taskID string, token string, 
timeout time.Duration) map[string]interface{} {
t.Helper()

var result map[string]interface{}

waitForCondition(t, "async task completion", timeout, 3*time.Second, func() bool {
resp := doGet(t, "/api/v1/tasks/"+taskID, token)
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
return false
}

if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
return false
}

status, ok := result["status"].(string)
return ok && (status == "completed" || status == "failed")
})

return result
}

// createAndCleanup creates a resource and registers cleanup.
func createAndCleanup(t *testing.T, createFn func() string, 
deletePath string, token string) string {
t.Helper()

id := createFn()

t.Cleanup(func() {
path := strings.ReplaceAll(deletePath, "{id}", id)
resp := doDelete(t, path, token)
resp.Body.Close()
})

return id
}

// futureDate returns a date N days in the future.
func futureDate(days int) string {
return time.Now().AddDate(0, 0, days).Format(time.RFC3339)
}

// pastDate returns a date N days in the past.
func pastDate(days int) string {
return time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
}

//Personal.AI order the ending
