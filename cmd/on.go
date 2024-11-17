package cmd

import (
	"github.com/eckertalex/keylight/internal/api"
	"github.com/eckertalex/keylight/internal/services"
	"github.com/spf13/cobra"
)

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Turn on the lights",
	Run: func(cmd *cobra.Command, args []string) {
		services.UpdateLightsSettings(api.LightDetail{On: 1})
	},
}

func init() {
	rootCmd.AddCommand(onCmd)
}
