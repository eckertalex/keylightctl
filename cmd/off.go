package cmd

import (
	"github.com/spf13/cobra"
)

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		updateLights(lights, LightDetail{On: 0})
	},
}

func init() {
	rootCmd.AddCommand(offCmd)
}
