package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Set via ldflags during build; see the Makefile's LDFLAGS.
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "version",
		Short:             "Print version information",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kubectl actuator version %s\n", Version)
			if GitCommit != "unknown" {
				fmt.Printf("Git commit: %s\n", GitCommit)
			}
			if BuildDate != "unknown" {
				fmt.Printf("Build date: %s\n", BuildDate)
			}
		},
	}

	return cmd
}
