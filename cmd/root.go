package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	lights  []Light
	rootCmd = &cobra.Command{
		Use:   "keylight",
		Short: "A CLI to manage your Elgato Key Light Air",
		Long: `keylight is a simple and efficient CLI tool to control your Elgato Key Light Air.
It provides commands to check the light status, turn it on or off, and adjust settings like brightness and temperature.

Configure your lights in a $HOME/.keylight.toml file and use keylight to manage them from the terminal.`,
	}
)

func Execute(version string) {
	rootCmd.Version = version
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.keylight.toml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("toml")
		viper.SetConfigName(".keylight")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %v\n", err)
		os.Exit(1)
	}

	if err := viper.UnmarshalKey("lights", &lights); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unmarshal lights: %v\n", err)
		os.Exit(1)
	}
}
