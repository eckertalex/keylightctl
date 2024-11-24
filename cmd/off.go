package cmd

import (
	"fmt"

	"github.com/eckertalex/keylightctl/internal/api"
	"github.com/eckertalex/keylightctl/internal/services"
	"github.com/spf13/cobra"
)

var (
	offLightName string
	offCmd       = &cobra.Command{
		Use:   "off",
		Short: "Turn off the lights",
		Run: func(cmd *cobra.Command, args []string) {
			if offLightName != "" {
				lightConfig := services.FindLightByName(lightsConfig, offLightName)

				if lightConfig == nil {
					availableLights := services.GetAvailableLightNames(lightsConfig)
					fmt.Printf("Light with name '%s' not found. Available lights: %s\n", offLightName, availableLights)
					return
				}

				lights := services.ToLights([]services.LightConfig{*lightConfig})
				services.UpdateLightsSettings(lights, api.LightDetail{On: 0})
				return
			}

			lights := services.ToLights(lightsConfig)
			services.UpdateLightsSettings(lights, api.LightDetail{On: 0})
		},
	}
)

func init() {
	offCmd.Flags().StringVarP(&offLightName, "light", "l", "", "Specify the light name to turn off")
	rootCmd.AddCommand(offCmd)
}
