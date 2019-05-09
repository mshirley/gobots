package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version of gobots",
	Long:  "ALL SOFTWARE HAS VERSIONS AND THIS ONE IS MINE",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("IT'S GOBOTS v0.0.1")
	},
}
