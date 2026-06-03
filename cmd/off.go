package cmd

import (
	"github.com/eckertalex/keylightctl/internal/keylight"
	"github.com/spf13/cobra"
)

var (
	offLightName string
	offCmd       = &cobra.Command{
		Use:   "off",
		Short: "Turn off the lights",
		Run: func(cmd *cobra.Command, args []string) {
			lights, ok := resolveLights(offLightName)
			if !ok {
				return
			}
			UpdateLightsSettings(lights, keylight.LightDetail{On: 0})
		},
	}
)

func init() {
	offCmd.Flags().StringVarP(&offLightName, "light", "l", "", "Specify the light name to turn off")

	rootCmd.AddCommand(offCmd)
}
