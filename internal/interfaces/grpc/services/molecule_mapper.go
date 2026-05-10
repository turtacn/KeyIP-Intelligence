package services

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/v1"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// domainToProto converts domain molecule to protobuf message
func domainToProto(mol *molecule.Molecule) *pb.Molecule {
	if mol == nil {
		return nil
	}

	// Convert properties to map[string]string
	propsMap := make(map[string]string)
	for _, prop := range mol.Properties {
		propsMap[prop.Name] = fmt.Sprintf("%v", prop.Value)
	}

	// Convert metadata to map[string]string
	metaMap := make(map[string]string)
	if mol.Metadata != nil {
		for k, v := range mol.Metadata {
			metaMap[k] = fmt.Sprintf("%v", v)
		}
	}

	return &pb.Molecule{
		MoleculeId:   mol.ID.String(),
		Smiles:       mol.SMILES,
		Inchi:        mol.InChI,
		Name:         mol.Name,
		MoleculeType: string(mol.Source),
		OledLayer:    "",
		Properties:   propsMap,
		Metadata:     metaMap,
		CreatedAt:    mol.CreatedAt.Unix(),
		UpdatedAt:    mol.UpdatedAt.Unix(),
	}
}

// mapDomainError maps domain errors to gRPC status codes
func mapDomainError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.IsNotFound(err):
		return status.Error(codes.NotFound, err.Error())
	case errors.IsValidation(err):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.IsConflict(err):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.IsUnauthorized(err):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
