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
