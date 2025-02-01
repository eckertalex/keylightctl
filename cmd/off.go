package cmd

import (
	"fmt"

	"github.com/eckertalex/keylightctl/internal/keylight"
	"github.com/spf13/cobra"
)

var (
	offLightName string
	offCmd       = &cobra.Command{
		Use:   "off",
		Short: "Turn off the lights",
		Run: func(cmd *cobra.Command, args []string) {
			if offLightName != "" {
				lightConfig := FindLightByName(lightsConfig, offLightName)

				if lightConfig == nil {
					availableLights := GetAvailableLightNames(lightsConfig)
					fmt.Printf("Light with name '%s' not found. Available lights: %s\n", offLightName, availableLights)
					return
				}

				lights := ToLights([]LightConfig{*lightConfig})
				UpdateLightsSettings(lights, keylight.LightDetail{On: 0})
				return
			}

			lights := ToLights(lightsConfig)
			UpdateLightsSettings(lights, keylight.LightDetail{On: 0})
		},
	}
)

func init() {
	offCmd.Flags().StringVarP(&offLightName, "light", "l", "", "Specify the light name to turn off")

	rootCmd.AddCommand(offCmd)
}
