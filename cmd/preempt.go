package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bpineau/cloud-floating-ip/pkg/operation"
	"github.com/bpineau/cloud-floating-ip/pkg/run"
)

var preemptCmd = &cobra.Command{
	Use:   "preempt",
	Short: "Preempt an IP address and route it to the instance",
	Long:  `Preempt an IP address and route it to the instance`,
	Run: func(cmd *cobra.Command, args []string) {
		run.Run(newCfiConfig(), operation.CfiPreempt)
	},
}

func init() {
	rootCmd.AddCommand(preemptCmd)
}
