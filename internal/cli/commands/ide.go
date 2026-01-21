package commands

import (
	"strings"

	"github.com/ozacod/cpx/internal/ide"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/spf13/cobra"
)

func IdeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ide [name]",
		Short: "Generate IDE configuration files",
		Long: `Generate configuration files for various IDEs/Editors.
Supported:
  - zed (default)

Example:
  cpx ide zed
  cpx ide zed -a config.yml -e "DEBUG=1"`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			target := "zed"
			var launchArgs []string

			// Check if the first argument is a known IDE
			if len(args) > 0 {
				target = args[0]
			}

			// Append arguments from flags
			flagArgs, _ := cmd.Flags().GetStringSlice("argument")
			launchArgs = append(launchArgs, flagArgs...)

			// Parse environment variables
			envFlags, _ := cmd.Flags().GetStringSlice("env")
			envVars := make(map[string]string)
			for _, e := range envFlags {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					envVars[parts[0]] = parts[1]
				} else if len(parts) == 1 && parts[0] != "" {
					envVars[parts[0]] = ""
				}
			}

			var err error
			switch target {
			case "zed":
				gen := ide.NewZedGenerator(launchArgs, envVars)
				err = gen.Generate()
			default:
				utils.PrintError("Unsupported IDE: %s", target)
				utils.PrintInfo("Supported: zed")
				return
			}

			if err != nil {
				utils.PrintError("Failed to generate configuration: %v", err)
			}
		},
	}

	cmd.Flags().StringSliceP("argument", "a", []string{}, "Arguments to pass to the debug launch configuration")
	cmd.Flags().StringSliceP("env", "e", []string{}, "Environment variables to set (NAME=VALUE)")

	return cmd
}
