package cmd

import (
	"github.com/eckertalex/keylightctl/internal/api"
	"github.com/eckertalex/keylightctl/internal/services"
	"github.com/spf13/cobra"
)

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Turn off the lights",
	Run: func(cmd *cobra.Command, args []string) {
		services.UpdateLightsSettings(api.LightDetail{On: 0})
	},
}

func init() {
	rootCmd.AddCommand(offCmd)
}
