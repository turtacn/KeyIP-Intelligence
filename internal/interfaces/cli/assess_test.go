package cli

import (
"os"
"strings"
"testing"
)

func TestNewAssessCmd(t *testing.T) {
cmd := NewAssessCmd()
if cmd == nil {
t.Fatal("NewAssessCmd should not return nil")
}
if cmd.Use != "assess" {
t.Errorf("expected Use='assess', got %s", cmd.Use)
}
}

func TestAssessPatentCmd_ValidSinglePatent(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "CN202110123456"})

// Note: This test validates that the command executes without error
// The output goes directly to stdout via fmt.Print, which can't be captured via cmd.SetOut
err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}
}

func TestAssessPatentCmd_ValidMultiplePatents(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "CN123,US456,EP789"})

err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}
}

func TestAssessPatentCmd_InvalidPatentNumber(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "INVALID123"})

err := cmd.Execute()
if err == nil {
t.Error("expected error for invalid patent number")
}
if !strings.Contains(err.Error(), "invalid patent number") {
t.Errorf("unexpected error: %v", err)
}
}

func TestAssessPatentCmd_MissingRequiredFlag(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{})

err := cmd.Execute()
if err == nil {
t.Error("expected error for missing required flag")
}
}

func TestAssessPatentCmd_InvalidDimension(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "CN123", "--dimensions", "invalid"})

err := cmd.Execute()
if err == nil {
t.Error("expected error for invalid dimension")
}
}

func TestAssessPatentCmd_JSONOutput(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "CN123", "--output", "json"})

// Note: Output goes to stdout via fmt.Print
// This test validates that JSON output format is accepted and executes without error
err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}
}

func TestAssessPatentCmd_CSVOutput(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "CN123", "--output", "csv"})

// Note: Output goes to stdout via fmt.Print
// This test validates that CSV output format is accepted and executes without error
err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}
}

func TestAssessPatentCmd_FileOutput(t *testing.T) {
tmpfile := t.TempDir() + "/output.txt"

cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "CN123", "--file", tmpfile})

err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}

content, err := os.ReadFile(tmpfile)
if err != nil {
t.Fatalf("failed to read output file: %v", err)
}

if len(content) == 0 {
t.Error("output file should not be empty")
}
}

func TestAssessPortfolioCmd_ValidPortfolio(t *testing.T) {
cmd := newAssessPortfolioCmd()
cmd.SetArgs([]string{"--portfolio-id", "pf-123"})

err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}
}

func TestFormatAssessOutput_Table(t *testing.T) {
result := &ValuationResult{
Patents: []PatentValuation{
{PatentNumber: "CN123", OverallScore: 85.5, RiskLevel: "LOW"},
},
}

output, err := formatAssessOutput(result, "stdout")
if err != nil {
t.Fatalf("formatting failed: %v", err)
}

if !strings.Contains(output, "CN123") {
t.Error("output should contain patent number")
}
}

func TestFormatAssessOutput_UnknownFormat(t *testing.T) {
result := &ValuationResult{}

_, err := formatAssessOutput(result, "unknown")
if err == nil {
t.Error("expected error for unknown format")
}
}

//Personal.AI order the ending
