package cli

import "testing"

func TestSearchCmd_Exists(t *testing.T) {
	if SearchCmd == nil {
		t.Error("SearchCmd should exist")
	}
}

//Personal.AI order the ending
