package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"cli/pkg/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "bmc-cli",
	Short: "BMC management CLI for hosting providers",
	Long: `A command-line interface for managing server BMC (Baseboard Management Controllers)
through a secure gateway system. Provides access to IPMI and Redfish interfaces
without exposing BMC ports directly.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.bmc-cli/config.yaml)")
	rootCmd.PersistentFlags().String("gateway-url", "", "gateway server URL")
	rootCmd.PersistentFlags().String("api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().String("token", "", "JWT token for authentication")

	viper.BindPFlag("gateway.url", rootCmd.PersistentFlags().Lookup("gateway-url"))
	viper.BindPFlag("auth.api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("auth.token", rootCmd.PersistentFlags().Lookup("token"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
}

func GetConfig() *config.Config {
	return cfg
}
