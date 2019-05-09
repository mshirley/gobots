package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "gobots",
	Short: "IT'S GOBOTS",
	Long:  "IT'S GOBOTS, IT'S GOBOTS",
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
