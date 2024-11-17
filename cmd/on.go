package cmd

import (
	"github.com/spf13/cobra"
)

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		updateLights(lights, LightDetail{On: 1})
	},
}

func init() {
	rootCmd.AddCommand(onCmd)
}
