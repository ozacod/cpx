package commands

import (
	"github.com/ozacod/cpx/internal/utils/git"
	"github.com/spf13/cobra"
)

func HooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Install git hooks",
		Long: `Install git hooks for code quality and automation:
   pre-commit   - Format code and run linters before commit
   pre-push     - Run tests and security checks before push
   commit-msg   - Validate commit message format
   post-merge   - Update dependencies if vcpkg.json changed`,
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install git hooks",
		Long:  "Install git hooks with default configuration (fmt, lint for pre-commit; test for pre-push).",
		RunE:  runHooksInstall,
	}
	cmd.AddCommand(installCmd)

	return cmd
}

func runHooksInstall(_ *cobra.Command, _ []string) error {
	// Use default hooks - no cpx.yaml needed
	return git.InstallHooksWithConfig([]string{"fmt", "lint"}, []string{"test"})
}
