package cmd

import (
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server management commands",
	Long:  "Commands for managing servers through their BMC interfaces",
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
