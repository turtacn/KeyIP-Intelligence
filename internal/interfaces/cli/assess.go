package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var AssessCmd = &cobra.Command{
	Use:   "assess",
	Short: "Assess patent infringement risk",
	RunE:  runAssess,
}

var (
	assessSMILES   string
	assessMolFile  string
	assessPatentID string
	assessVerbose  bool
)

func init() {
	AssessCmd.Flags().StringVar(&assessSMILES, "smiles", "", "Molecule SMILES")
	AssessCmd.Flags().StringVar(&assessMolFile, "molecule-file", "", "Molecule file")
	AssessCmd.Flags().StringVar(&assessPatentID, "patent-id", "", "Patent ID")
	AssessCmd.Flags().BoolVarP(&assessVerbose, "verbose", "v", false, "Verbose")
	RootCmd.AddCommand(AssessCmd)
}

func runAssess(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if assessSMILES == "" && assessMolFile == "" {
		return fmt.Errorf("either --smiles or --molecule-file required")
	}

	smiles := assessSMILES
	if assessMolFile != "" {
		data, err := os.ReadFile(assessMolFile)
		if err != nil {
			return fmt.Errorf("read file error: %w", err)
		}
		smiles = strings.TrimSpace(string(data))
	}

	fmt.Printf("Assessing: %s\n", smiles)
	_ = ctx
	return nil
}

//Personal.AI order the ending
