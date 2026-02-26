package molecule

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestMoleculeStatus_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   MoleculeStatus
		expected string
	}{
		{MoleculeStatusPending, "pending"},
		{MoleculeStatusActive, "active"},
		{MoleculeStatusArchived, "archived"},
		{MoleculeStatusDeleted, "deleted"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("MoleculeStatus.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestMoleculeStatus_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status MoleculeStatus
		valid  bool
	}{
		{MoleculeStatusPending, true},
		{MoleculeStatusActive, true},
		{MoleculeStatusArchived, true},
		{MoleculeStatusDeleted, true},
		{MoleculeStatus("unknown"), false},
	}

	for _, tt := range tests {
		if got := tt.status.IsValid(); got != tt.valid {
			t.Errorf("MoleculeStatus.IsValid() = %v, want %v for %s", got, tt.valid, tt.status)
		}
	}
}

func TestMoleculeSource_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source MoleculeSource
		valid  bool
	}{
		{SourcePatent, true},
		{SourceLiterature, true},
		{SourceExperiment, true},
		{SourcePrediction, true},
		{SourceManual, true},
		{MoleculeSource("unknown"), false},
	}

	for _, tt := range tests {
		if got := tt.source.IsValid(); got != tt.valid {
			t.Errorf("MoleculeSource.IsValid() = %v, want %v for %s", got, tt.valid, tt.source)
		}
	}
}

func TestNewMolecule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		smiles    string
		source    MoleculeSource
		sourceRef string
		wantErr   bool
	}{
		{
			name:      "valid_simple_smiles",
			smiles:    "c1ccccc1",
			source:    SourceManual,
			sourceRef: "test",
			wantErr:   false,
		},
		{
			name:      "valid_complex_smiles",
			smiles:    "CC1=CC(=C(C=C1)N(C2=CC=C(C=C2)C3=CC=CC=C3)C4=CC=C(C=C4)C5=CC=CC=C5)C",
			source:    SourcePatent,
			sourceRef: "US1234567",
			wantErr:   false,
		},
		{
			name:      "empty_smiles",
			smiles:    "",
			source:    SourceManual,
			sourceRef: "test",
			wantErr:   true,
		},
		{
			name:      "exceeds_max_length",
			smiles:    strings.Repeat("C", 10001),
			source:    SourceManual,
			sourceRef: "test",
			wantErr:   true,
		},
		{
			name:      "unbalanced_parentheses",
			smiles:    "C(C(C",
			source:    SourceManual,
			sourceRef: "test",
			wantErr:   true,
		},
		{
			name:      "illegal_characters",
			smiles:    "C!@#$",
			source:    SourceManual,
			sourceRef: "test",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMolecule(tt.smiles, tt.source, tt.sourceRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMolecule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == nil {
					t.Error("NewMolecule() returned nil molecule")
				} else {
					if got.GetStatus() != MoleculeStatusPending {
						t.Errorf("NewMolecule() status = %v, want %v", got.GetStatus(), MoleculeStatusPending)
					}
					if got.ID == uuid.Nil {
						t.Error("NewMolecule() returned empty ID")
					}
					if got.GetSMILES() != tt.smiles {
						t.Errorf("NewMolecule() smiles = %v, want %v", got.GetSMILES(), tt.smiles)
					}
					if got.GetSource() != tt.source {
						t.Errorf("NewMolecule() source = %v, want %v", got.GetSource(), tt.source)
					}
					if got.GetSourceRef() != tt.sourceRef {
						t.Errorf("NewMolecule() sourceRef = %v, want %v", got.GetSourceRef(), tt.sourceRef)
					}
				}
			}
		})
	}
}

func TestMolecule_Validate(t *testing.T) {
	t.Parallel()
	m := newTestMolecule(t)

	tests := []struct {
		name    string
		setup   func(*Molecule)
		wantErr bool
	}{
		{
			name:    "valid_complete",
			setup:   func(m *Molecule) {},
			wantErr: false,
		},
		{
			name:    "missing_id",
			setup:   func(m *Molecule) { m.ID = uuid.Nil },
			wantErr: true,
		},
		{
			name:    "missing_smiles",
			setup:   func(m *Molecule) { m.SMILES = "" },
			wantErr: true,
		},
		{
			name:    "negative_weight",
			setup:   func(m *Molecule) { m.MolecularWeight = -1.0 },
			wantErr: true,
		},
		{
			name:    "invalid_inchikey_format",
			setup:   func(m *Molecule) { m.InChIKey = "INVALID" },
			wantErr: true,
		},
		{
			name:    "valid_inchikey",
			setup:   func(m *Molecule) { m.InChIKey = "AAAAAAAAAAAAAA-BBBBBBBBBB-C" },
			wantErr: false,
		},
		{
			name:    "invalid_confidence",
			setup:   func(m *Molecule) { m.PropertiesMap["test"] = &MolecularProperty{Name: "test", Confidence: 1.5} },
			wantErr: true,
		},
		{
			name:    "negative_confidence",
			setup:   func(m *Molecule) { m.PropertiesMap["test"] = &MolecularProperty{Name: "test", Confidence: -0.1} },
			wantErr: true,
		},
		{
			name:    "invalid_formula",
			setup:   func(m *Molecule) { m.MolecularFormula = "C(H)3" },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Deep copy
			localM := *m
			localM.PropertiesMap = make(map[string]*MolecularProperty)
			localM.FingerprintsMap = make(map[FingerprintType]*Fingerprint)
			localM.Tags = make([]string, 0)
			localM.Metadata = make(map[string]any)

			tt.setup(&localM)
			if err := localM.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Molecule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMolecule_StateTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Activate_Success", func(t *testing.T) {
		m := newTestMolecule(t)
		m.InChIKey = "AAAAAAAAAAAAAA-BBBBBBBBBB-C"
		err := m.Activate()
		if err != nil {
			t.Fatalf("Activate() failed: %v", err)
		}
		if m.GetStatus() != MoleculeStatusActive {
			t.Errorf("Status = %v, want %v", m.GetStatus(), MoleculeStatusActive)
		}
		if m.GetVersion() != 2 {
			t.Errorf("Version = %d, want 2", m.GetVersion())
		}
	})

	t.Run("Activate_NoInChIKey", func(t *testing.T) {
		m := newTestMolecule(t)
		m.InChIKey = ""
		err := m.Activate()
		if err == nil {
			t.Error("Activate() succeeded without InChIKey")
		}
	})

	t.Run("Activate_WrongStatus", func(t *testing.T) {
		m := newActiveMolecule(t)
		err := m.Activate()
		if err == nil {
			t.Error("Activate() succeeded from Active status")
		}
	})

	t.Run("Archive_Success", func(t *testing.T) {
		m := newActiveMolecule(t)
		err := m.Archive()
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}
		if m.GetStatus() != MoleculeStatusArchived {
			t.Errorf("Status = %v, want %v", m.GetStatus(), MoleculeStatusArchived)
		}
	})

	t.Run("Archive_WrongStatus", func(t *testing.T) {
		m := newTestMolecule(t)
		err := m.Archive()
		if err == nil {
			t.Error("Archive() succeeded from Pending status")
		}
	})

	t.Run("MarkDeleted_FromActive", func(t *testing.T) {
		m := newActiveMolecule(t)
		err := m.MarkDeleted()
		if err != nil {
			t.Fatalf("MarkDeleted() failed: %v", err)
		}
		if m.GetStatus() != MoleculeStatusDeleted {
			t.Errorf("Status = %v, want %v", m.GetStatus(), MoleculeStatusDeleted)
		}
	})

	t.Run("MarkDeleted_FromArchived", func(t *testing.T) {
		m := newActiveMolecule(t)
		m.Archive()
		err := m.MarkDeleted()
		if err != nil {
			t.Fatalf("MarkDeleted() failed: %v", err)
		}
		if m.GetStatus() != MoleculeStatusDeleted {
			t.Errorf("Status = %v, want %v", m.GetStatus(), MoleculeStatusDeleted)
		}
	})

	t.Run("MarkDeleted_FromDeleted", func(t *testing.T) {
		m := newActiveMolecule(t)
		m.MarkDeleted()
		err := m.MarkDeleted()
		if err == nil {
			t.Error("MarkDeleted() succeeded from Deleted status")
		}
	})

	t.Run("MarkDeleted_FromPending", func(t *testing.T) {
		m := newTestMolecule(t)
		err := m.MarkDeleted()
		if err == nil {
			t.Error("MarkDeleted() succeeded from Pending status")
		}
	})
}

func TestMolecule_SetStructureIdentifiers(t *testing.T) {
	t.Parallel()
	m := newTestMolecule(t)

	canonical := "c1ccccc1"
	inchi := "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H"
	inchiKey := "UHOVQNZJYSORNB-UHFFFAOYSA-N"
	formula := "C6H6"
	weight := 78.11

	err := m.SetStructureIdentifiers(canonical, inchi, inchiKey, formula, weight)
	if err != nil {
		t.Fatalf("SetStructureIdentifiers() failed: %v", err)
	}

	if m.GetCanonicalSMILES() != canonical {
		t.Errorf("CanonicalSmiles = %v, want %v", m.GetCanonicalSMILES(), canonical)
	}
	if m.GetInChI() != inchi {
		t.Errorf("InChI = %v, want %v", m.GetInChI(), inchi)
	}
	if m.GetInChIKey() != inchiKey {
		t.Errorf("InChIKey = %v, want %v", m.GetInChIKey(), inchiKey)
	}
	if m.GetMolecularFormula() != formula {
		t.Errorf("MolecularFormula = %v, want %v", m.GetMolecularFormula(), formula)
	}
	if m.GetMolecularWeight() != weight {
		t.Errorf("MolecularWeight = %v, want %v", m.GetMolecularWeight(), weight)
	}

	// Error cases
	if err := m.SetStructureIdentifiers("", inchi, inchiKey, formula, weight); err == nil {
		t.Error("SetStructureIdentifiers allowed empty canonical SMILES")
	}
	if err := m.SetStructureIdentifiers(canonical, "", inchiKey, formula, weight); err == nil {
		t.Error("SetStructureIdentifiers allowed empty InChI")
	}
	if err := m.SetStructureIdentifiers(canonical, inchi, "INVALID", formula, weight); err == nil {
		t.Error("SetStructureIdentifiers allowed invalid InChIKey")
	}
	if err := m.SetStructureIdentifiers(canonical, inchi, inchiKey, formula, -1.0); err == nil {
		t.Error("SetStructureIdentifiers allowed negative weight")
	}
}

func TestMolecule_FingerprintManagement(t *testing.T) {
	t.Parallel()
	m := newTestMolecule(t)
	fp := newTestFingerprint(t, FingerprintMorgan)

	// Add
	if err := m.AddFingerprint(fp); err != nil {
		t.Fatalf("AddFingerprint() failed: %v", err)
	}

	// Verify
	if !m.HasFingerprint(FingerprintMorgan) {
		t.Error("HasFingerprint(Morgan) returned false")
	}
	got, ok := m.GetFingerprint(FingerprintMorgan)
	if !ok || got != fp {
		t.Error("GetFingerprint(Morgan) returned incorrect value")
	}

	// Not exists
	if m.HasFingerprint(FingerprintMACCS) {
		t.Error("HasFingerprint(MACCS) returned true unexpectedly")
	}

	// Override
	fp2 := newTestFingerprint(t, FingerprintMorgan)
	if err := m.AddFingerprint(fp2); err != nil {
		t.Fatalf("AddFingerprint() override failed: %v", err)
	}
	got2, _ := m.GetFingerprint(FingerprintMorgan)
	if got2 != fp2 {
		t.Error("AddFingerprint() override failed to update value")
	}

	// Nil fingerprint
	if err := m.AddFingerprint(nil); err == nil {
		t.Error("AddFingerprint(nil) succeeded")
	}
}

func TestMolecule_PropertyManagement(t *testing.T) {
	t.Parallel()
	m := newTestMolecule(t)
	prop := &MolecularProperty{Name: "test", Value: 1.0, Confidence: 0.9}

	// Add
	if err := m.AddProperty(prop); err != nil {
		t.Fatalf("AddProperty() failed: %v", err)
	}

	// Get
	got, ok := m.GetProperty("test")
	if !ok || got != prop {
		t.Error("GetProperty() returned incorrect value")
	}

	// Not exists
	if _, ok := m.GetProperty("nonexistent"); ok {
		t.Error("GetProperty(nonexistent) returned true")
	}

	// Override
	prop2 := &MolecularProperty{Name: "test", Value: 2.0, Confidence: 0.8}
	if err := m.AddProperty(prop2); err != nil {
		t.Fatalf("AddProperty() override failed: %v", err)
	}
	got2, _ := m.GetProperty("test")
	if got2.Value != 2.0 {
		t.Error("AddProperty() override failed to update value")
	}

	// Errors
	if err := m.AddProperty(nil); err == nil {
		t.Error("AddProperty(nil) succeeded")
	}
	if err := m.AddProperty(&MolecularProperty{Name: ""}); err == nil {
		t.Error("AddProperty(empty name) succeeded")
	}
	if err := m.AddProperty(&MolecularProperty{Name: "bad", Confidence: 1.1}); err == nil {
		t.Error("AddProperty(invalid confidence) succeeded")
	}
}

func TestMolecule_TagManagement(t *testing.T) {
	t.Parallel()
	m := newTestMolecule(t)

	// Add
	if err := m.AddTag("tag1"); err != nil {
		t.Fatalf("AddTag() failed: %v", err)
	}

	tags := m.GetTags()
	if len(tags) != 1 || tags[0] != "tag1" {
		t.Errorf("GetTags() = %v, want [tag1]", tags)
	}

	// Duplicate
	if err := m.AddTag("tag1"); err != nil {
		t.Fatalf("AddTag() duplicate failed: %v", err)
	}
	if len(m.GetTags()) != 1 {
		t.Error("AddTag() duplicate increased count")
	}

	// Add second
	if err := m.AddTag("tag2"); err != nil {
		t.Fatalf("AddTag() failed: %v", err)
	}
	if len(m.GetTags()) != 2 {
		t.Error("GetTags() count incorrect after adding second tag")
	}

	// Remove
	m.RemoveTag("tag1")
	tags = m.GetTags()
	if len(tags) != 1 || tags[0] != "tag2" {
		t.Errorf("GetTags() after remove = %v, want [tag2]", tags)
	}

	// Remove non-existent
	m.RemoveTag("nonexistent")
	if len(m.GetTags()) != 1 {
		t.Error("RemoveTag() non-existent changed count")
	}

	// Errors
	if err := m.AddTag(""); err == nil {
		t.Error("AddTag(empty) succeeded")
	}
	if err := m.AddTag(strings.Repeat("a", 65)); err == nil {
		t.Error("AddTag(too long) succeeded")
	}
}

func TestMolecule_GettersIsolation(t *testing.T) {
	t.Parallel()
	m := newTestMolecule(t)
	m.AddTag("tag1")
	m.SetMetadata("key", "value")

	// Tags
	tags := m.GetTags()
	tags[0] = "modified"
	if m.GetTags()[0] == "modified" {
		t.Error("GetTags() returned reference to internal slice")
	}

	// Metadata
	meta := m.GetMetadata()
	meta["key"] = "modified"
	if m.GetMetadata()["key"] == "modified" {
		t.Error("GetMetadata() returned reference to internal map")
	}
}

func TestMolecule_ConvenienceMethods(t *testing.T) {
	t.Parallel()
	m := newTestMolecule(t)
	if !m.IsPending() { t.Error("IsPending returned false") }
	if m.IsActive() { t.Error("IsActive returned true") }

	m.InChIKey = "UHOVQNZJYSORNB-UHFFFAOYSA-N"
	m.Activate()
	if !m.IsActive() { t.Error("IsActive returned false") }
	if m.IsPending() { t.Error("IsPending returned true") }

	m.Archive()
	if !m.IsArchived() { t.Error("IsArchived returned false") }

	m.MarkDeleted()
	if !m.IsDeleted() { t.Error("IsDeleted returned false") }

	str := m.String()
	if !strings.Contains(str, "Molecule{") || !strings.Contains(str, "deleted") {
		t.Errorf("String() format incorrect: %s", str)
	}
}

// Helpers

func newTestMolecule(t *testing.T) *Molecule {
	m, err := NewMolecule("c1ccccc1", SourceManual, "test")
	if err != nil {
		t.Fatalf("Failed to create test molecule: %v", err)
	}
	return m
}

func newActiveMolecule(t *testing.T) *Molecule {
	m := newTestMolecule(t)
	m.SetStructureIdentifiers("c1ccccc1", "InChI=...", "AAAAAAAAAAAAAA-BBBBBBBBBB-C", "C6H6", 78.11)
	if err := m.Activate(); err != nil {
		t.Fatalf("Failed to activate test molecule: %v", err)
	}
	return m
}

func newTestFingerprint(t *testing.T, fpType FingerprintType) *Fingerprint {
	fp, err := NewBitFingerprint(fpType, make([]byte, 256), 2048, 2)
	if err != nil {
		if fpType == FingerprintMACCS {
			fp, err = NewBitFingerprint(fpType, make([]byte, 21), 166, 0)
		}
	}
	if err != nil {
		t.Fatalf("Failed to create test fingerprint: %v", err)
	}
	return fp
}

//Personal.AI order the ending
