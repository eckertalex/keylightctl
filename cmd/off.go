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
				light := services.FindLightByName(lights, offLightName)

				if light == nil {
					availableLights := services.GetAvailableLightNames(lights)
					fmt.Printf("Light with name '%s' not found. Available lights: %s\n", offLightName, availableLights)
					return
				}

				services.UpdateLightsSettings([]services.Light{*light}, api.LightDetail{On: 0})
				return
			}

			services.UpdateLightsSettings(lights, api.LightDetail{On: 0})
		},
	}
)

func init() {
	offCmd.Flags().StringVarP(&offLightName, "light", "l", "", "Specify the light name to turn off")
	rootCmd.AddCommand(offCmd)
}
