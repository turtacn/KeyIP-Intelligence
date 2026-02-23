package cli

import (
"bytes"
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

var stdout bytes.Buffer
cmd.SetOut(&stdout)

err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}

output := stdout.String()
if !strings.Contains(output, "CN202110123456") {
t.Error("output should contain patent number")
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

var stdout bytes.Buffer
cmd.SetOut(&stdout)

err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}

output := stdout.String()
if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
t.Error("output should be valid JSON")
}
}

func TestAssessPatentCmd_CSVOutput(t *testing.T) {
cmd := newAssessPatentCmd()
cmd.SetArgs([]string{"--patent-number", "CN123", "--output", "csv"})

var stdout bytes.Buffer
cmd.SetOut(&stdout)

err := cmd.Execute()
if err != nil {
t.Fatalf("execution failed: %v", err)
}

output := stdout.String()
if !strings.Contains(output, "Patent Number") {
t.Error("CSV output should contain header")
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
