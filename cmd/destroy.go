package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bpineau/cloud-floating-ip/pkg/operation"
	"github.com/bpineau/cloud-floating-ip/pkg/run"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Delete the routes managed by cloud-floating-ip",
	Long:  `Delete the routes managed by cloud-floating-ip`,
	Run: func(cmd *cobra.Command, args []string) {
		run.Run(newCfiConfig(), operation.CfiDestroy)
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}
