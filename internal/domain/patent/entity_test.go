package patent

import (
	"testing"
	"time"
)

func TestPatentStatus_String(t *testing.T) {
	tests := []struct {
		s    PatentStatus
		want string
	}{
		{PatentStatusDraft, "Draft"},
		{PatentStatusFiled, "Filed"},
		{PatentStatusPublished, "Published"},
		{PatentStatusUnderExamination, "UnderExamination"},
		{PatentStatusGranted, "Granted"},
		{PatentStatusRejected, "Rejected"},
		{PatentStatusWithdrawn, "Withdrawn"},
		{PatentStatusExpired, "Expired"},
		{PatentStatusInvalidated, "Invalidated"},
		{PatentStatusLapsed, "Lapsed"},
		{PatentStatusUnknown, "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("PatentStatus.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestPatentStatus_IsValid(t *testing.T) {
	if !PatentStatusDraft.IsValid() {
		t.Error("Draft should be valid")
	}
	if PatentStatus(0).IsValid() {
		t.Error("Status 0 should be invalid")
	}
	if PatentStatus(255).IsValid() {
		t.Error("Status 255 should be invalid")
	}
}

func TestPatentStatus_IsActive(t *testing.T) {
	if !PatentStatusFiled.IsActive() {
		t.Error("Filed should be active")
	}
	if !PatentStatusGranted.IsActive() {
		t.Error("Granted should be active")
	}
	if PatentStatusExpired.IsActive() {
		t.Error("Expired should not be active")
	}
}

func TestPatentStatus_IsTerminal(t *testing.T) {
	if PatentStatusDraft.IsTerminal() {
		t.Error("Draft should not be terminal")
	}
	if !PatentStatusRejected.IsTerminal() {
		t.Error("Rejected should be terminal")
	}
}

func TestPatentOffice_IsValid(t *testing.T) {
	if !OfficeCNIPA.IsValid() {
		t.Error("CNIPA should be valid")
	}
	if PatentOffice("UNKNOWN").IsValid() {
		t.Error("UNKNOWN should be invalid")
	}
}

func TestIPCClassification(t *testing.T) {
	ipc := IPCClassification{Full: "C09K 11/06"}
	if err := ipc.Validate(); err != nil {
		t.Errorf("Expected valid, got %v", err)
	}
	if ipc.String() != "C09K 11/06" {
		t.Errorf("String() mismatch")
	}
	ipcEmpty := IPCClassification{Full: ""}
	if err := ipcEmpty.Validate(); err == nil {
		t.Error("Expected error for empty full code")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestPatentDate_RemainingLifeYears(t *testing.T) {
	now := time.Now().UTC()
	future := now.AddDate(10, 0, 0)
	past := now.AddDate(-1, 0, 0)

	pd := PatentDate{ExpiryDate: &future}
	remaining := pd.RemainingLifeYears()
	if remaining <= 9.0 || remaining >= 11.0 {
		t.Errorf("Expected ~10 years remaining, got %f", remaining)
	}

	pdPast := PatentDate{ExpiryDate: &past}
	if pdPast.RemainingLifeYears() != 0 {
		t.Error("Expected 0 for past expiry")
	}

	pdNil := PatentDate{ExpiryDate: nil}
	if pdNil.RemainingLifeYears() != 0 {
		t.Error("Expected 0 for nil expiry")
	}
}

func TestPatentDate_Validate(t *testing.T) {
	filing := time.Now().UTC()
	pub := filing.AddDate(0, 6, 0)
	grant := filing.AddDate(2, 0, 0)
	expiry := filing.AddDate(20, 0, 0)

	t.Run("Valid Sequence", func(t *testing.T) {
		pd := PatentDate{
			FilingDate:      &filing,
			PublicationDate: &pub,
			GrantDate:       &grant,
			ExpiryDate:      &expiry,
		}
		if err := pd.Validate(); err != nil {
			t.Errorf("Expected valid, got %v", err)
		}
	})

	t.Run("Pub Before Filing", func(t *testing.T) {
		before := filing.AddDate(0, -1, 0)
		pd := PatentDate{
			FilingDate:      &filing,
			PublicationDate: &before,
		}
		if err := pd.Validate(); err == nil {
			t.Error("Expected error")
		}
	})

	t.Run("Grant Before Pub", func(t *testing.T) {
		before := pub.AddDate(0, -1, 0)
		pd := PatentDate{
			FilingDate:      &filing,
			PublicationDate: &pub,
			GrantDate:       &before,
		}
		if err := pd.Validate(); err == nil {
			t.Error("Expected error")
		}
	})

	t.Run("Expiry Before Grant", func(t *testing.T) {
		before := grant.AddDate(0, -1, 0)
		pd := PatentDate{
			FilingDate:      &filing,
			PublicationDate: &pub,
			GrantDate:       &grant,
			ExpiryDate:      &before,
		}
		if err := pd.Validate(); err == nil {
			t.Error("Expected error")
		}
	})

	t.Run("Nil Filing", func(t *testing.T) {
		pd := PatentDate{FilingDate: nil}
		if err := pd.Validate(); err == nil {
			t.Error("Expected error")
		}
	})
}

func newTestPatent() *Patent {
	filingDate := time.Now().UTC().AddDate(-1, 0, 0)
	p, _ := NewPatent("CN115123456B", "Test OLED Material", OfficeCNIPA, filingDate)
	p.Applicants = []Applicant{{Name: "OLED Corp", Country: "CN", Type: "company"}}
	p.Inventors = []Inventor{{Name: "Zhang San", Country: "CN", Affiliation: "OLED Corp"}}
	return p
}

func TestNewPatent(t *testing.T) {
	filingDate := time.Now().UTC()
	p, err := NewPatent("US1234567", "OLED Device", OfficeUSPTO, filingDate)
	if err != nil {
		t.Fatalf("NewPatent failed: %v", err)
	}
	if p.ID == "" {
		t.Error("ID should not be empty")
	}
	if p.Status != PatentStatusFiled {
		t.Error("Initial status should be Filed")
	}
	if p.Version != 1 {
		t.Error("Initial version should be 1")
	}

	// Error cases
	if _, err := NewPatent("", "Title", OfficeUSPTO, filingDate); err == nil {
		t.Error("Expected error for empty number")
	}
	if _, err := NewPatent("N", "", OfficeUSPTO, filingDate); err == nil {
		t.Error("Expected error for empty title")
	}
}

func TestPatent_StateTransitions(t *testing.T) {
	p := newTestPatent()
	now := time.Now().UTC()

	t.Run("Full Lifecycle Success", func(t *testing.T) {
		// Filed -> Published
		if err := p.Publish(now); err != nil {
			t.Errorf("Publish failed: %v", err)
		}
		if p.Status != PatentStatusPublished || p.Version != 2 {
			t.Error("Publish transition failed")
		}

		// Published -> Examination
		if err := p.EnterExamination(); err != nil {
			t.Errorf("EnterExamination failed: %v", err)
		}
		if p.Status != PatentStatusUnderExamination || p.Version != 3 {
			t.Error("EnterExamination transition failed")
		}

		// Examination -> Granted
		expiry := now.AddDate(20, 0, 0)
		if err := p.Grant(now, expiry); err != nil {
			t.Errorf("Grant failed: %v", err)
		}
		if p.Status != PatentStatusGranted || p.Version != 4 {
			t.Error("Grant transition failed")
		}

		// Granted -> Expired
		if err := p.Expire(); err != nil {
			t.Errorf("Expire failed: %v", err)
		}
		if p.Status != PatentStatusExpired || p.Version != 5 {
			t.Error("Expire transition failed")
		}
	})

	t.Run("Illegal Transitions", func(t *testing.T) {
		p2 := newTestPatent() // Filed
		if err := p2.EnterExamination(); err == nil {
			t.Error("Should not transition from Filed directly to Examination")
		}

		p2.Publish(now) // Published
		if err := p2.Grant(now, now); err == nil {
			t.Error("Should not transition from Published directly to Grant")
		}
	})

	t.Run("Terminal States", func(t *testing.T) {
		p3 := newTestPatent()
		p3.Publish(now)
		p3.EnterExamination()
		if err := p3.Reject(); err != nil {
			t.Error("Reject failed")
		}
		if !p3.Status.IsTerminal() {
			t.Error("Rejected should be terminal")
		}
	})
}

func TestPatent_Molecules(t *testing.T) {
	p := newTestPatent()
	molID := "MOL-001"

	if err := p.AddMolecule(molID); err != nil {
		t.Errorf("AddMolecule failed: %v", err)
	}
	if !p.HasMolecule(molID) {
		t.Error("HasMolecule failed")
	}
	if err := p.AddMolecule(molID); err == nil {
		t.Error("Expected error for duplicate molecule")
	}

	if err := p.RemoveMolecule(molID); err != nil {
		t.Errorf("RemoveMolecule failed: %v", err)
	}
	if p.HasMolecule(molID) {
		t.Error("Molecule should have been removed")
	}
	if err := p.RemoveMolecule("NON-EXISTENT"); err == nil {
		t.Error("Expected error for non-existent molecule removal")
	}
}

func TestPatent_Citations(t *testing.T) {
	p := newTestPatent()
	cit := "US9876543B2"

	p.AddCitation(cit)
	if len(p.Cites) != 1 || p.Cites[0] != cit {
		t.Error("AddCitation failed")
	}

	p.AddCitedBy(cit)
	if len(p.CitedBy) != 1 || p.CitedBy[0] != cit {
		t.Error("AddCitedBy failed")
	}
}

func TestPatent_Claims(t *testing.T) {
	p := newTestPatent()
	text := "This is a valid claim text of sufficient length."
	claims := ClaimSet{
		{Number: 1, Text: text, Type: ClaimTypeIndependent, Category: ClaimCategoryProduct},
	}

	if err := p.SetClaims(claims); err != nil {
		t.Errorf("SetClaims failed: %v", err)
	}
	if p.ClaimCount() != 1 {
		t.Errorf("ClaimCount failed: %d", p.ClaimCount())
	}
	if p.IndependentClaimCount() != 1 {
		t.Error("IndependentClaimCount failed")
	}
}

//Personal.AI order the ending
