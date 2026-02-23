package cli

import (
	"testing"
)

func TestRootCmd_HasVersion(t *testing.T) {
	if RootCmd.Version == "" {
		t.Error("version not set")
	}
}

func TestRootCmd_HasUse(t *testing.T) {
	if RootCmd.Use == "" {
		t.Error("Use not set")
	}
}

//Personal.AI order the ending
