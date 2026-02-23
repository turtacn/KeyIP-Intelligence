package client

import "context"

// MoleculesClient handles molecule-related API operations
type MoleculesClient struct {
client *Client
}

// Molecule represents a molecule structure
type Molecule struct {
ID       string `json:"id"`
SMILES   string `json:"smiles"`
InChI    string `json:"inchi"`
Name     string `json:"name"`
}

// Search searches molecules by structure similarity
func (mc *MoleculesClient) Search(ctx context.Context, smiles string, threshold float64) ([]Molecule, error) {
var result struct {
Molecules []Molecule `json:"molecules"`
}
err := mc.client.post(ctx, "/api/v1/molecules/search", map[string]interface{}{
"smiles": smiles,
"threshold": threshold,
}, &result)
return result.Molecules, err
}

//Personal.AI order the ending
