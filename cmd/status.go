package cmd

import (
	"github.com/spf13/cobra"
)

var (
	statusLightName string
	statusCmd       = &cobra.Command{
		Use:   "status",
		Short: "Get the current status of all configured lights",
		Run: func(cmd *cobra.Command, args []string) {
			lights, ok := resolveLights(statusLightName)
			if !ok {
				return
			}
			GetLightsSettings(lights)
		},
	}
)

func init() {
	statusCmd.Flags().StringVarP(&statusLightName, "light", "l", "", "Specify the light name")

	rootCmd.AddCommand(statusCmd)
}
