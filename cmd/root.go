package cmd

import (
	"fmt"
	"os"

	"github.com/eckertalex/keylightctl/internal/services"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Lights  []services.Light
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "keylightctl",
		Short: "A CLI to manage your Elgato Key Light Air",
	}
)

func Execute(version string) {
	rootCmd.Version = version
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.keylightctl.toml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("toml")
		viper.SetConfigName(".keylightctl")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %v\n", err)
		os.Exit(1)
	}

	if err := viper.UnmarshalKey("lights", &Lights); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unmarshal lights: %v\n", err)
		os.Exit(1)
	}
}
