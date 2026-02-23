#!/bin/bash
set -e

# CLI files
cat > internal/interfaces/cli/assess_test.go << 'EOF'
package cli

import "testing"

func TestAssessCmd_Exists(t *testing.T) {
if AssessCmd == nil {
t.Error("AssessCmd should exist")
}
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/cli/lifecycle.go << 'EOF'
package cli

import (
"context"
"fmt"
"time"

"github.com/spf13/cobra"
)

var LifecycleCmd = &cobra.Command{
Use:   "lifecycle",
Short: "Manage patent lifecycle",
}

var deadlinesCmd = &cobra.Command{
Use:   "deadlines",
Short: "List deadlines",
RunE:  runDeadlines,
}

var lifecycleDays int

func init() {
LifecycleCmd.AddCommand(deadlinesCmd)
deadlinesCmd.Flags().IntVar(&lifecycleDays, "days", 90, "Days ahead")
RootCmd.AddCommand(LifecycleCmd)
}

func runDeadlines(cmd *cobra.Command, args []string) error {
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
fmt.Printf("Checking deadlines for next %d days\n", lifecycleDays)
_ = ctx
return nil
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/cli/lifecycle_test.go << 'EOF'
package cli

import "testing"

func TestLifecycleCmd_Exists(t *testing.T) {
if LifecycleCmd == nil {
t.Error("LifecycleCmd should exist")
}
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/cli/report.go << 'EOF'
package cli

import (
"context"
"fmt"
"time"

"github.com/spf13/cobra"
)

var ReportCmd = &cobra.Command{
Use:   "report",
Short: "Generate reports",
}

var ftoCmd = &cobra.Command{
Use:   "fto",
Short: "FTO report",
RunE:  runFTO,
}

var reportOutput string

func init() {
ReportCmd.AddCommand(ftoCmd)
ftoCmd.Flags().StringVar(&reportOutput, "output", "report.pdf", "Output path")
RootCmd.AddCommand(ReportCmd)
}

func runFTO(cmd *cobra.Command, args []string) error {
ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
defer cancel()
fmt.Printf("Generating FTO report: %s\n", reportOutput)
_ = ctx
return nil
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/cli/report_test.go << 'EOF'
package cli

import "testing"

func TestReportCmd_Exists(t *testing.T) {
if ReportCmd == nil {
t.Error("ReportCmd should exist")
}
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/cli/search.go << 'EOF'
package cli

import (
"context"
"fmt"
"time"

"github.com/spf13/cobra"
)

var SearchCmd = &cobra.Command{
Use:   "search",
Short: "Search patents",
}

var searchPatentsCmd = &cobra.Command{
Use:   "patents",
Short: "Search patents",
RunE:  runSearchPatents,
}

var searchQuery string

func init() {
SearchCmd.AddCommand(searchPatentsCmd)
searchPatentsCmd.Flags().StringVarP(&searchQuery, "query", "q", "", "Query")
RootCmd.AddCommand(SearchCmd)
}

func runSearchPatents(cmd *cobra.Command, args []string) error {
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if searchQuery == "" {
return fmt.Errorf("query required")
}
fmt.Printf("Searching: %s\n", searchQuery)
_ = ctx
return nil
}

//Personal.AI order the ending
EOF

cat > internal/interfaces/cli/search_test.go << 'EOF'
package cli

import "testing"

func TestSearchCmd_Exists(t *testing.T) {
if SearchCmd == nil {
t.Error("SearchCmd should exist")
}
}

//Personal.AI order the ending
EOF

echo "✓ Created 7 CLI files (1 already done)"
