package cli

import (
	"bytes"
	"testing"
)

func TestNewRootCmd_Structure(t *testing.T) {
	cmd := NewRootCmd(nil)
	if cmd == nil {
		t.Fatal("NewRootCmd should return a command")
	}

	// Verify Use field
	if cmd.Use != "keyip" {
		t.Errorf("expected Use='keyip', got %q", cmd.Use)
	}

	// Verify Short field
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	// Verify Long field
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

func TestNewRootCmd_SubcommandRegistration(t *testing.T) {
	cmd := NewRootCmd(nil)
	subs := cmd.Commands()

	// Check minimum number of subcommands
	if len(subs) < 5 {
		t.Errorf("expected at least 5 subcommands, got %d", len(subs))
	}

	// Verify required subcommands
	expectedSubs := []string{"assess", "lifecycle", "report", "search", "version"}
	subNames := make(map[string]bool)
	for _, sub := range subs {
		subNames[sub.Use] = true
	}

	for _, name := range expectedSubs {
		if !subNames[name] {
			t.Errorf("expected subcommand %q not found", name)
		}
	}
}

func TestNewRootCmd_GlobalFlags(t *testing.T) {
	cmd := NewRootCmd(nil)

	// Check config flag
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("config flag should exist")
	}

	// Check verbose flag
	verboseFlag := cmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("verbose flag should exist")
	}

	// Check no-color flag
	noColorFlag := cmd.PersistentFlags().Lookup("no-color")
	if noColorFlag == nil {
		t.Error("no-color flag should exist")
	}
}

func TestNewRootCmd_ConfigFlagDefault(t *testing.T) {
	cmd := NewRootCmd(nil)

	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Fatal("config flag should exist")
	}

	if configFlag.DefValue != "configs/config.yaml" {
		t.Errorf("config flag default should be 'configs/config.yaml', got %q", configFlag.DefValue)
	}
}

func TestNewRootCmd_VerboseFlag(t *testing.T) {
	cmd := NewRootCmd(nil)

	verboseFlag := cmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Fatal("verbose flag should exist")
	}

	// Check shorthand
	if verboseFlag.Shorthand != "v" {
		t.Errorf("verbose flag shorthand should be 'v', got %q", verboseFlag.Shorthand)
	}

	// Check default value
	if verboseFlag.DefValue != "false" {
		t.Errorf("verbose flag default should be 'false', got %q", verboseFlag.DefValue)
	}
}

func TestNewRootCmd_NoColorFlag(t *testing.T) {
	cmd := NewRootCmd(nil)

	noColorFlag := cmd.PersistentFlags().Lookup("no-color")
	if noColorFlag == nil {
		t.Fatal("no-color flag should exist")
	}

	if noColorFlag.DefValue != "false" {
		t.Errorf("no-color flag default should be 'false', got %q", noColorFlag.DefValue)
	}
}

func TestVersionCmd_Output(t *testing.T) {
	cmd := newVersionCmd()
	if cmd == nil {
		t.Fatal("newVersionCmd should return a command")
	}

	// The version command uses fmt.Printf which writes to stdout
	// We just verify that the command executes without error
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Verify build variables are accessible
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
	if GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}
}

func TestVersionCmd_EmptyBuildInfo(t *testing.T) {
	// Save original values
	origVersion := Version
	origBuildTime := BuildTime
	origGitCommit := GitCommit

	// Set to empty/unknown
	Version = "0.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"

	defer func() {
		// Restore original values
		Version = origVersion
		BuildTime = origBuildTime
		GitCommit = origGitCommit
	}()

	cmd := newVersionCmd()
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Verify variables were set correctly
	if Version != "0.0.0" {
		t.Errorf("Version should be '0.0.0', got %q", Version)
	}
	if BuildTime != "unknown" {
		t.Errorf("BuildTime should be 'unknown', got %q", BuildTime)
	}
}

func TestExecute_Success(t *testing.T) {
	// Test with help flag (should succeed)
	deps := &CLIDeps{}
	cmd := NewRootCmd(deps)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}

func TestExecute_UnknownSubcommand(t *testing.T) {
	deps := &CLIDeps{}
	cmd := NewRootCmd(deps)
	cmd.SetArgs([]string{"unknownsubcommand"})

	// Capture stderr to prevent output during test
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestCLIDeps_NilCheck(t *testing.T) {
	// Should not panic with nil deps
	cmd := NewRootCmd(nil)
	if cmd == nil {
		t.Error("NewRootCmd should handle nil deps")
	}
}

func TestCLIDeps_NoColorConfig(t *testing.T) {
	deps := &CLIDeps{NoColor: true}
	if !deps.NoColor {
		t.Error("NoColor should be true")
	}

	deps.NoColor = false
	if deps.NoColor {
		t.Error("NoColor should be false")
	}
}

func TestRootCmd_HasVersion(t *testing.T) {
	if RootCmd == nil {
		t.Fatal("RootCmd should not be nil")
	}
	if RootCmd.Version == "" {
		t.Error("version not set")
	}
}

func TestRootCmd_HasUse(t *testing.T) {
	if RootCmd == nil {
		t.Fatal("RootCmd should not be nil")
	}
	if RootCmd.Use == "" {
		t.Error("Use not set")
	}
}

func TestBuildVariables(t *testing.T) {
	// Verify build variables have default values
	if Version == "" {
		t.Error("Version should have a default value")
	}
	if BuildTime == "" {
		t.Error("BuildTime should have a default value")
	}
	if GitCommit == "" {
		t.Error("GitCommit should have a default value")
	}
}

//Personal.AI order the ending
