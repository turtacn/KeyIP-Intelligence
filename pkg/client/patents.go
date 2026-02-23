package client

import "context"

// PatentsClient handles patent-related API operations
type PatentsClient struct {
client *Client
}

// Patent represents a patent document
type Patent struct {
ID            string `json:"id"`
PatentNumber  string `json:"patent_number"`
Title         string `json:"title"`
Applicant     string `json:"applicant"`
PublicationDate string `json:"publication_date"`
}

// Search searches patents by query
func (pc *PatentsClient) Search(ctx context.Context, query string) ([]Patent, error) {
var result struct {
Patents []Patent `json:"patents"`
}
err := pc.client.post(ctx, "/api/v1/patents/search", map[string]interface{}{
"query": query,
}, &result)
return result.Patents, err
}

// Get retrieves a patent by ID
func (pc *PatentsClient) Get(ctx context.Context, id string) (*Patent, error) {
var patent Patent
err := pc.client.get(ctx, "/api/v1/patents/"+id, &patent)
return &patent, err
}

//Personal.AI order the ending
