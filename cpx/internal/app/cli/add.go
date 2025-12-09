package cli

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

// ...
var addRunVcpkgCommandFunc func([]string) error

// AddCmd creates the add command
func AddCmd(runVcpkgCommand func([]string) error) *cobra.Command {
	addRunVcpkgCommandFunc = runVcpkgCommand

	cmd := &cobra.Command{
		Use:   "add [package]",
		Short: "Add a dependency",
		Long:  "Add a dependency. Passes through to vcpkg add command and prints usage info.",
		RunE:  runAdd,
		Args:  cobra.MinimumNArgs(1),
	}

	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	if err := requireVcpkgProject("cpx add"); err != nil {
		return err
	}

	// Directly pass all arguments to vcpkg add command
	// cpx add <pkg> -> vcpkg add port <pkg>
	vcpkgArgs := []string{"add", "port"}
	vcpkgArgs = append(vcpkgArgs, args...)

	if err := addRunVcpkgCommandFunc(vcpkgArgs); err != nil {
		return err
	}

	// Print usage info for the first package
	if len(args) > 0 {
		pkgName := args[0]
		if !strings.HasPrefix(pkgName, "-") {
			printUsageInfo(pkgName)
		}
	}

	return nil
}

// printUsageInfo fetches and prints usage info from GitHub
func printUsageInfo(pkgName string) {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/microsoft/vcpkg/master/ports/%s/usage", pkgName))
	if err != nil || resp.StatusCode != 200 {
		return
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	content := strings.TrimSpace(string(bytes))
	if content != "" {
		fmt.Printf("\n%sUSAGE INFO FOR %s:%s\n", Cyan, pkgName, Reset)
		fmt.Println(content)
		fmt.Println()
	}
}
