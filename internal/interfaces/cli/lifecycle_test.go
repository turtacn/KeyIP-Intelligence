package cli

import "testing"

func TestLifecycleCmd_Exists(t *testing.T) {
	if LifecycleCmd == nil {
		t.Error("LifecycleCmd should exist")
	}
}

//Personal.AI order the ending
