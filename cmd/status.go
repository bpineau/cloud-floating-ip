package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bpineau/cloud-floating-ip/pkg/operation"
	"github.com/bpineau/cloud-floating-ip/pkg/run"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display the status of the instance (owner or standby)",
	Long: `Display the status of the instance:
owner when the floating IP address route to the instance, standby otherwise.`,
	Run: func(cmd *cobra.Command, args []string) {
		run.Run(newCfiConfig(), operation.CfiStatus)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
