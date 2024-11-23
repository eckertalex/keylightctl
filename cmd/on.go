package cmd

import (
	"github.com/eckertalex/keylightctl/internal/api"
	"github.com/eckertalex/keylightctl/internal/services"
	"github.com/spf13/cobra"
)

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Turn on the lights",
	Run: func(cmd *cobra.Command, args []string) {
		services.UpdateLightsSettings(Lights, api.LightDetail{On: 1})
	},
}

func init() {
	rootCmd.AddCommand(onCmd)
}
