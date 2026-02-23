package cli

import "testing"

func TestAssessCmd_Exists(t *testing.T) {
	if AssessCmd == nil {
		t.Error("AssessCmd should exist")
	}
}

//Personal.AI order the ending
