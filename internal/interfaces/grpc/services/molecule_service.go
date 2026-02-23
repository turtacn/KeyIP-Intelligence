package services

import "context"

type MoleculeServiceServer struct{}

func NewMoleculeServiceServer() *MoleculeServiceServer {
	return &MoleculeServiceServer{}
}

func (s *MoleculeServiceServer) SearchSimilar(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, nil
}

//Personal.AI order the ending
