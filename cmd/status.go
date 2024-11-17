package cmd

import (
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the current status of all configured lights",
	Run: func(cmd *cobra.Command, args []string) {
		getStatus(lights)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
