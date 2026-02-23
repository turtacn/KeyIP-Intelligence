package services

import "context"

type PatentServiceServer struct{}

func NewPatentServiceServer() *PatentServiceServer {
	return &PatentServiceServer{}
}

func (s *PatentServiceServer) SearchPatents(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, nil
}

//Personal.AI order the ending
