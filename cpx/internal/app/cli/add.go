package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
		Long:  "Add a dependency. Passes through to vcpkg add command and attempts to update CMakeLists.txt.",
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

	// Smart Add: Try to automate CMake integration for the first package
	// We only handle the first arg as 'package' for now to keep it simple
	if len(args) > 0 {
		// Ignore flags
		pkgName := args[0]
		if !strings.HasPrefix(pkgName, "-") {
			return smartAdd(pkgName)
		}
	}

	return nil
}

// smartAdd attempts to find usage info and update CMakeLists.txt
func smartAdd(pkgName string) error {
	var content string

	// 1. Fetch usage info from GitHub (remote)
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/microsoft/vcpkg/master/ports/%s/usage", pkgName))
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		bytes, _ := io.ReadAll(resp.Body)
		content = string(bytes)
		fmt.Printf("%s[Fetched usage from GitHub]%s\n", Cyan, Reset)
	} else {
		// If remote fetch fails, we just silently skip usage display
		// The user might be offline or the package doesn't have a usage file
		return nil
	}

	fmt.Printf("\n%sUSAGE INFO FOR %s:%s\n", Cyan, pkgName, Reset)
	fmt.Println(strings.TrimSpace(content))
	fmt.Println()

	// 2. Parse CMake commands
	findPackageRegex := regexp.MustCompile(`find_package\s*\(\s*` + regexp.QuoteMeta(pkgName) + `.*?\)|find_package\s*\(\s*\w+.*?\s+CONFIG.*?\)|find_package\s*\(\s*\w+.*?\s+REQUIRED.*?\)|find_package\s*\(\s*[^)]+\s*\)`)
	// Heuristic: target_link_libraries might mention 'main', 'libs', or include namespaced targets like parsed::target
	// We'll simplisticly look for the string target_link_libraries followed by the package name or namespaced version
	// Actually, usage files usually say "target_link_libraries(main PRIVATE namespace::lib)".
	// We want to extract the "namespace::lib" part.

	// Better approach: Extract the raw commands from usage text using regex provided they look like CMake
	findMatches := findPackageRegex.FindAllString(content, -1)

	// For target_link_libraries, usage files come in many shapes.
	// Commonly: "target_link_libraries(main PRIVATE match::match)"
	linkRegex := regexp.MustCompile(`target_link_libraries\s*\(\s*\w+\s+\w+\s+(.*?)\s*\)`)
	linkMatches := linkRegex.FindStringSubmatch(content)

	targetsToLink := ""
	if len(linkMatches) > 1 {
		targetsToLink = linkMatches[1]
	} else if strings.Contains(content, pkgName+"::"+pkgName) {
		targetsToLink = pkgName + "::" + pkgName // Guess common pattern
	}

	if len(findMatches) == 0 && targetsToLink == "" {
		fmt.Printf("%sCould not parse CMake commands to auto-update CMakeLists.txt.%s\n", Yellow, Reset)
		return nil
	}

	// 3. Update CMakeLists.txt
	cmakePath := "CMakeLists.txt"
	cmakeContentBytes, err := os.ReadFile(cmakePath)
	if err != nil {
		return nil // No CMakeLists.txt found
	}
	cmakeContent := string(cmakeContentBytes)

	updated := false

	// Identify the exact find_package command to use (prefer one with CONFIG if multiple)
	findCmd := ""
	if len(findMatches) > 0 {
		findCmd = findMatches[0]
		// Prefer the match that actually contains the package name if possible
		for _, m := range findMatches {
			if strings.Contains(m, pkgName) {
				findCmd = m
				break
			}
		}
	}

	// Inject find_package
	if findCmd != "" && !strings.Contains(cmakeContent, findCmd) {
		// Insert after matching existing find_package or after project()
		if strings.Contains(cmakeContent, "find_package(") {
			// Find last find_package
			lastIdx := strings.LastIndex(cmakeContent, "find_package(")
			endOfLine := strings.Index(cmakeContent[lastIdx:], "\n")
			if endOfLine != -1 {
				insertPos := lastIdx + endOfLine + 1
				cmakeContent = cmakeContent[:insertPos] + findCmd + "\n" + cmakeContent[insertPos:]
				updated = true
				fmt.Printf("%s+ Added: %s%s\n", Green, findCmd, Reset)
			}
		} else if strings.Contains(cmakeContent, "project(") {
			// Insert after project() block (heuristic: find project( line, then next empty line or set commands)
			// Simplest: Find project(...) and look for next empty line
			projIdx := strings.Index(cmakeContent, "project(")
			endProj := strings.Index(cmakeContent[projIdx:], ")")
			if endProj != -1 {
				insertPos := projIdx + endProj + 1
				// skip usually 2-3 lines of sets
				nextLine := strings.Index(cmakeContent[insertPos:], "\n\n")
				if nextLine != -1 {
					insertPos += nextLine + 2
				} else {
					insertPos += 1 // just next line
				}
				cmakeContent = cmakeContent[:insertPos] + findCmd + "\n" + cmakeContent[insertPos:]
				updated = true
				fmt.Printf("%s+ Added: %s%s\n", Green, findCmd, Reset)
			}
		}
	}

	// Inject target_link_libraries
	if targetsToLink != "" {
		// Look for existing target_link_libraries
		// Assume standard cpx structure: project_name is often the target, or look for add_executable/add_library target
		// We'll just append to the FIRST target_link_libraries call we find in the file for now.
		// A more robust way is to find the project name and use that.

		// Find project name from project(...)
		// Support names with hyphens and optional quotes: project("my-app") or project(my-app)
		projNameRegex := regexp.MustCompile(`project\s*\(\s*"?([\w-]+)"?`)
		projMatch := projNameRegex.FindStringSubmatch(cmakeContent)
		if len(projMatch) > 1 {
			projTarget := projMatch[1] // e.g. ff-ex

			// Try to find target_link_libraries(projTarget ... PRIVATE ...) OR target_link_libraries(${PROJECT_NAME} ... PRIVATE ...)
			// We construct a regex that matches either the literal name or ${PROJECT_NAME}
			// We use (?s) to allow . to match newlines, supporting multiline target_link_libraries
			targetPattern := `(?:` + regexp.QuoteMeta(projTarget) + `|\$\{PROJECT_NAME\})`
			linkCmdRegex := regexp.MustCompile(`(?s)target_link_libraries\s*\(\s*` + targetPattern + `\s+PRIVATE\s+(.*?)\)`)
			existingLinkMatch := linkCmdRegex.FindStringSubmatch(cmakeContent)

			if len(existingLinkMatch) > 0 {
				if !strings.Contains(existingLinkMatch[1], targetsToLink) {
					// Replace the match
					fullMatch := existingLinkMatch[0]
					newLinkCmd := strings.Replace(fullMatch, ")", " "+targetsToLink+")", 1)
					cmakeContent = strings.Replace(cmakeContent, fullMatch, newLinkCmd, 1)
					updated = true
					fmt.Printf("%s+ Linked: %s to %s%s\n", Green, targetsToLink, projTarget, Reset)
				}
			} else {
				// No target_link_libraries found, create one
				// Look for add_executable OR add_library definition for this target
				// Regex to capture the command and its content up to the closing regex
				// This is tricky with regex due to nested parens, but usually these are top-level.
				// We'll look for "add_executable(projTarget" or "add_library(projTarget"
				// and just append the new command after the project definition block or at the end of file?
				// Better anchor: Look for add_executable(projTarget or add_library(projTarget

				defRegex := regexp.MustCompile(`(?s)(add_executable|add_library)\s*\(\s*` + regexp.QuoteMeta(projTarget) + `\s+.*?\)(?:\s+)?`)
				defMatch := defRegex.FindStringIndex(cmakeContent)

				if defMatch != nil {
					// Found definition, insert after it
					insertPos := defMatch[1] // End of match
					newLinkCmd := fmt.Sprintf("\n\ntarget_link_libraries(%s PRIVATE %s)\n", projTarget, targetsToLink)
					cmakeContent = cmakeContent[:insertPos] + newLinkCmd + cmakeContent[insertPos:]
					updated = true
					fmt.Printf("%s+ Created: target_link_libraries(%s PRIVATE %s)%s\n", Green, projTarget, targetsToLink, Reset)
				} else {
					// Fallback: Append to end of file if we extracted a project name but couldn't find its definition (rare)
					// Or maybe it's defined differently.
					// Safe bet: just append to end of file
					cmakeContent += fmt.Sprintf("\n\ntarget_link_libraries(%s PRIVATE %s)\n", projTarget, targetsToLink)
					updated = true
					fmt.Printf("%s+ Appended: target_link_libraries(%s PRIVATE %s) to end of file%s\n", Green, projTarget, targetsToLink, Reset)
				}
			}
		}
	}

	if updated {
		if err := os.WriteFile(cmakePath, []byte(cmakeContent), 0644); err != nil {
			return fmt.Errorf("failed to update CMakeLists.txt: %w", err)
		}
		fmt.Printf("%s✓ Updated CMakeLists.txt successfully%s\n", Green, Reset)
	} else {
		fmt.Printf("%s(CMakeLists.txt not updated automatically - check usage info above)%s\n", Yellow, Reset)
	}

	return nil
}
