package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "1.0.0"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of dost",
	Long:  `All software has versions. This is dost's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dost version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
