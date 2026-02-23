package client

import "context"

// LifecycleClient handles patent lifecycle operations
type LifecycleClient struct {
client *Client
}

// Deadline represents a patent deadline
type Deadline struct {
PatentNumber string `json:"patent_number"`
Type         string `json:"type"`
DueDate      string `json:"due_date"`
Status       string `json:"status"`
}

// GetDeadlines retrieves upcoming deadlines
func (lc *LifecycleClient) GetDeadlines(ctx context.Context, days int) ([]Deadline, error) {
var result struct {
Deadlines []Deadline `json:"deadlines"`
}
err := lc.client.get(ctx, "/api/v1/lifecycle/deadlines", &result)
return result.Deadlines, err
}

//Personal.AI order the ending
