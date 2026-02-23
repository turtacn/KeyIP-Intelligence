package cli

import "testing"

func TestReportCmd_Exists(t *testing.T) {
	if ReportCmd == nil {
		t.Error("ReportCmd should exist")
	}
}

//Personal.AI order the ending
