package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "create bot client binary",
	Long:  "CREATE BOT CLIENT",
	Run: func(cmd *cobra.Command, args []string) {
		createClient()
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
}

func createClient() {
	fmt.Println("CREATING BOT CLIENT")
}
