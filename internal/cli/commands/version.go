package commands

import (
	"fmt"

	"github.com/ozacod/cpx/internal/utils"
	"github.com/spf13/cobra"
)

func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of cpx",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cpx version %s\n", utils.Version)
		},
	}
}
