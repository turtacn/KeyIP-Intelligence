package chem_extractor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type MockPubChemClient struct {
	mock.Mock
}

func (m *MockPubChemClient) SearchByName(ctx context.Context, name string) (*PubChemCompound, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PubChemCompound), args.Error(1)
}

func (m *MockPubChemClient) SearchByCAS(ctx context.Context, cas string) (*PubChemCompound, error) {
	args := m.Called(ctx, cas)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PubChemCompound), args.Error(1)
}

func (m *MockPubChemClient) SearchBySMILES(ctx context.Context, smiles string) (*PubChemCompound, error) {
	args := m.Called(ctx, smiles)
	return args.Get(0).(*PubChemCompound), args.Error(1)
}

func (m *MockPubChemClient) GetCompound(ctx context.Context, cid int) (*PubChemCompound, error) {
	args := m.Called(ctx, cid)
	return args.Get(0).(*PubChemCompound), args.Error(1)
}

type MockRDKitService struct {
	mock.Mock
}

func (m *MockRDKitService) ValidateSMILES(smiles string) (bool, error) {
	args := m.Called(smiles)
	return args.Bool(0), args.Error(1)
}

func (m *MockRDKitService) CanonicalizeSMILES(smiles string) (string, error) {
	args := m.Called(smiles)
	return args.String(0), args.Error(1)
}

func (m *MockRDKitService) SMILESToInChI(smiles string) (string, error) {
	args := m.Called(smiles)
	return args.String(0), args.Error(1)
}

func (m *MockRDKitService) SMILESToMolecularFormula(smiles string) (string, error) {
	args := m.Called(smiles)
	return args.String(0), args.Error(1)
}

func (m *MockRDKitService) ComputeMolecularWeight(smiles string) (float64, error) {
	args := m.Called(smiles)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockRDKitService) SMILESToInChIKey(smiles string) (string, error) {
	args := m.Called(smiles)
	return args.String(0), args.Error(1)
}

// MockLogger again...
type MockLogger struct {
	logging.Logger
	mock.Mock
}

func (m *MockLogger) Warn(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func TestResolve_CASNumber_PubChemFallback(t *testing.T) {
	pc := new(MockPubChemClient)
	rdkit := new(MockRDKitService)
	logger := new(MockLogger)
	config := ResolverConfig{ExternalLookupEnabled: true}

	resolver := NewEntityResolver(pc, rdkit, nil, nil, config, logger)

	pc.On("SearchByCAS", mock.Anything, "50-78-2").Return(&PubChemCompound{
		CanonicalSMILES: "CC(=O)Oc1ccccc1C(=O)O",
		IUPACName:       "2-acetoxybenzoic acid",
	}, nil)

	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{Text: "50-78-2", EntityType: EntityCASNumber})
	assert.NoError(t, err)
	assert.True(t, res.IsResolved)
	assert.Equal(t, "CC(=O)Oc1ccccc1C(=O)O", res.SMILES)
}

func TestResolve_SMILES_Valid(t *testing.T) {
	rdkit := new(MockRDKitService)
	logger := new(MockLogger)
	config := ResolverConfig{ExternalLookupEnabled: false}

	resolver := NewEntityResolver(nil, rdkit, nil, nil, config, logger)

	smiles := "CC(=O)Oc1ccccc1C(=O)O"
	rdkit.On("ValidateSMILES", smiles).Return(true, nil)
	rdkit.On("CanonicalizeSMILES", smiles).Return(smiles, nil)
	rdkit.On("SMILESToInChI", smiles).Return("InChI=...", nil)
	rdkit.On("SMILESToInChIKey", smiles).Return("BSYNRYMUTXBXSQ-UHFFFAOYSA-N", nil)
	rdkit.On("ComputeMolecularWeight", smiles).Return(180.16, nil)
	rdkit.On("SMILESToMolecularFormula", smiles).Return("C9H8O4", nil)

	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{Text: smiles, EntityType: EntitySMILES})
	assert.NoError(t, err)
	assert.True(t, res.IsResolved)
	assert.Equal(t, 180.16, res.MolecularWeight)
}

func TestResolve_Batch(t *testing.T) {
	pc := new(MockPubChemClient)
	logger := new(MockLogger)
	config := ResolverConfig{ExternalLookupEnabled: true}

	resolver := NewEntityResolver(pc, nil, nil, nil, config, logger)

	pc.On("SearchByCAS", mock.Anything, "50-78-2").Return(&PubChemCompound{CanonicalSMILES: "A"}, nil)
	pc.On("SearchByCAS", mock.Anything, "123-45-6").Return(&PubChemCompound{CanonicalSMILES: "B"}, nil)

	batch := []*RawChemicalEntity{
		{Text: "50-78-2", EntityType: EntityCASNumber},
		{Text: "123-45-6", EntityType: EntityCASNumber},
	}

	results, err := resolver.ResolveBatch(context.Background(), batch)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(results))
	assert.Equal(t, "A", results[0].SMILES)
	assert.Equal(t, "B", results[1].SMILES)
}
//Personal.AI order the ending
