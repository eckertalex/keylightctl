package cmd

import (
	"fmt"

	"github.com/eckertalex/keylightctl/internal/api"
	"github.com/eckertalex/keylightctl/internal/services"
	"github.com/eckertalex/keylightctl/internal/utils"
	"github.com/spf13/cobra"
)

var (
	onBrightness  *int
	onTemperature *int
	onLightName   string
	onCmd         = &cobra.Command{
		Use:   "on",
		Short: "Turn on the lights",
		Run: func(cmd *cobra.Command, args []string) {
			settings := api.LightDetail{
				On: 1,
			}

			if cmd.Flags().Changed("brightness") {
				if err := utils.ValidateBrightness(*onBrightness); err != nil {
					fmt.Printf("Invalid brightness: %v\n", err)
					return
				}
				settings.Brightness = *onBrightness
			}

			if cmd.Flags().Changed("temperature") {
				if err := utils.ValidateTemperature(*onTemperature); err != nil {
					fmt.Printf("Invalid temperature: %v\n", err)
					return
				}
				settings.Temperature = utils.KelvinToMired(*onTemperature)
			}

			if onLightName != "" {
				lightConfig := services.FindLightByName(lightsConfig, onLightName)

				if lightConfig == nil {
					availableLights := services.GetAvailableLightNames(lightsConfig)
					fmt.Printf("Light with name '%s' not found. Available lights: %s\n", onLightName, availableLights)
					return
				}

				lights := services.ToLights([]services.LightConfig{*lightConfig})
				services.UpdateLightsSettings(lights, settings)
				return
			}

			lights := services.ToLights(lightsConfig)
			services.UpdateLightsSettings(lights, settings)
		},
	}
)

func init() {
	onBrightness = new(int)
	onTemperature = new(int)

	onCmd.Flags().IntVarP(onBrightness, "brightness", "b", 0, "Brightness percentage (0-100)")
	onCmd.Flags().IntVarP(onTemperature, "temperature", "t", 0, "Color temperature in Kelvin (2900-7000)")
	onCmd.Flags().StringVarP(&onLightName, "light", "l", "", "Specify the light name")
	rootCmd.AddCommand(onCmd)
}
