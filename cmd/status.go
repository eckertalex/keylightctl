package cmd

import (
	"fmt"

	"github.com/eckertalex/keylightctl/internal/services"
	"github.com/spf13/cobra"
)

var (
	statusLightName string
	statusCmd       = &cobra.Command{
		Use:   "status",
		Short: "Get the current status of all configured lights",
		Run: func(cmd *cobra.Command, args []string) {
			if statusLightName != "" {
				lightConfig := services.FindLightByName(lightsConfig, statusLightName)

				if lightConfig == nil {
					availableLights := services.GetAvailableLightNames(lightsConfig)
					fmt.Printf("Light with name '%s' not found. Available lights: %s\n", statusLightName, availableLights)
					return
				}

				lights := services.ToLights([]services.LightConfig{*lightConfig})
				services.GetLightsSettings(lights)
				return
			}

			lights := services.ToLights(lightsConfig)
			services.GetLightsSettings(lights)
		},
	}
)

func init() {
	statusCmd.Flags().StringVarP(&statusLightName, "light", "l", "", "Specify the light name")
	rootCmd.AddCommand(statusCmd)
}
