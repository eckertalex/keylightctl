package cmd

import (
	"fmt"

	"github.com/eckertalex/keylightctl/internal/keylight"
	"github.com/spf13/cobra"
)

var (
	statusLightName string
	statusCmd       = &cobra.Command{
		Use:   "status",
		Short: "Get the current status of all configured lights",
		Run: func(cmd *cobra.Command, args []string) {
			if statusLightName != "" {
				lightConfig := FindLightByName(lightsConfig, statusLightName)

				if lightConfig == nil {
					availableLights := GetAvailableLightNames(lightsConfig)
					fmt.Printf("Light with name '%s' not found. Available lights: %s\n", statusLightName, availableLights)
					return
				}

				lights := ToLights([]keylight.LightConfig{*lightConfig})
				GetLightsSettings(lights)
				return
			}

			lights := ToLights(lightsConfig)
			GetLightsSettings(lights)
		},
	}
)

func init() {
	statusCmd.Flags().StringVarP(&statusLightName, "light", "l", "", "Specify the light name")

	rootCmd.AddCommand(statusCmd)
}
