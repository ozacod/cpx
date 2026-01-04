package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ozacod/cpx/internal/config"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/ozacod/cpx/internal/utils/colors"
	"github.com/spf13/cobra"
)

func SelfUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self-upgrade",
		Short: "Upgrade cpx to the latest version",
		Long:  "Upgrade cpx to the latest version from GitHub releases.",
		RunE:  runSelfUpgrade,
	}

	vcpkgCmd := &cobra.Command{
		Use:   "vcpkg",
		Short: "Update vcpkg installation to the latest version",
		Long:  "Run git pull in the vcpkg directory to update it to the latest version.",
		RunE:  runSelfUpgradeVcpkg,
	}
	cmd.AddCommand(vcpkgCmd)

	return cmd
}

func runSelfUpgrade(_ *cobra.Command, args []string) error {
	SelfUpgrade(args)
	return nil
}

func SelfUpgrade(_ []string) {
	fmt.Printf("%s Checking for updates...%s\n", colors.Cyan, colors.Reset)

	// Get latest version from GitHub releases API
	resp, err := http.Get("https://api.github.com/repos/ozacod/cpx/releases/latest")
	if err != nil {
		utils.PrintError("failed to check for updates: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			utils.PrintError("error closing response body: %v", err)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("%s  No releases found. This may be the first version.%s\n", colors.Yellow, colors.Reset)
		fmt.Printf("   Repository: https://github.com/ozacod/cpx\n")
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		utils.PrintError("failed to check for updates (status %d): %s", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		utils.PrintError("failed to parse release info: %v", err)
		os.Exit(1)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := utils.Version

	if latestVersion == currentVersion {
		fmt.Printf("%s You're already running the latest version (%s)%s\n", colors.Green, currentVersion, colors.Reset)
		return
	}

	fmt.Printf("%s New version available: %s  %s%s\n", colors.Yellow, currentVersion, latestVersion, colors.Reset)
	fmt.Printf("   Release: %s\n", release.HTMLURL)

	// Determine platform and architecture
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var binaryName string
	switch goos {
	case "darwin":
		binaryName = fmt.Sprintf("cpx-darwin-%s", goarch)
	case "linux":
		binaryName = fmt.Sprintf("cpx-linux-%s", goarch)
	case "windows":
		binaryName = fmt.Sprintf("cpx-windows-%s.exe", goarch)
	default:
		utils.PrintError("unsupported platform: %s", goos)
		os.Exit(1)
	}

	downloadURL := fmt.Sprintf("https://github.com/ozacod/cpx/releases/download/%s/%s", release.TagName, binaryName)
	fmt.Printf("%s Downloading %s...%s\n", colors.Cyan, binaryName, colors.Reset)

	// Download the new binary
	resp, err = http.Get(downloadURL)
	if err != nil {
		utils.PrintError("failed to download: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			utils.PrintError("error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != 200 {
		utils.PrintError("download failed with status %d", resp.StatusCode)
		os.Exit(1)
	}

	binaryData, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.PrintError("failed to read download: %v", err)
		os.Exit(1)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		utils.PrintError("failed to get executable path: %v", err)
		os.Exit(1)
	}
	execPath, _ = filepath.EvalSymlinks(execPath)

	// Write to temp file first
	tempPath := execPath + ".new"
	if err := os.WriteFile(tempPath, binaryData, 0755); err != nil {
		// Try writing to temp directory instead
		tempPath = filepath.Join(os.TempDir(), "cpx-new")
		if err := os.WriteFile(tempPath, binaryData, 0755); err != nil {
			utils.PrintError("failed to write binary: %v", err)
			os.Exit(1)
		}
		fmt.Printf("%s Downloaded to %s%s\n", colors.Green, tempPath, colors.Reset)
		fmt.Printf("\nTo complete the upgrade, run:\n")
		fmt.Printf("  sudo mv %s %s\n", tempPath, execPath)
		return
	}

	// Remove old binary and rename new one
	if err := os.Remove(execPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove old binary: %v\n", err)
	}
	if err := os.Rename(tempPath, execPath); err != nil {
		utils.PrintError("failed to replace binary: %v", err)
		fmt.Printf("\nTo complete manually, run:\n")
		fmt.Printf("  sudo mv %s %s\n", tempPath, execPath)
		os.Exit(1)
	}

	fmt.Printf("%s Successfully upgraded to %s!%s\n", colors.Green, latestVersion, colors.Reset)
	fmt.Printf("  Run %scpx version%s to verify.\n", colors.Cyan, colors.Reset)
}

// runSelfUpgradeVcpkg updates vcpkg by running git pull in its directory
func runSelfUpgradeVcpkg(_ *cobra.Command, _ []string) error {
	// Load global config to get vcpkg root
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	vcpkgRoot := cfg.VcpkgRoot
	if vcpkgRoot == "" {
		// Try VCPKG_ROOT environment variable
		vcpkgRoot = os.Getenv("VCPKG_ROOT")
	}

	if vcpkgRoot == "" {
		return fmt.Errorf("vcpkg root not configured. Run 'cpx config set-vcpkg-root <path>' or set VCPKG_ROOT environment variable")
	}

	// Check if directory exists
	if _, err := os.Stat(vcpkgRoot); os.IsNotExist(err) {
		return fmt.Errorf("vcpkg directory not found: %s", vcpkgRoot)
	}

	// Check if it's a git repository
	gitDir := filepath.Join(vcpkgRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("vcpkg directory is not a git repository: %s", vcpkgRoot)
	}

	fmt.Printf("%s Updating vcpkg in %s...%s\n", colors.Cyan, vcpkgRoot, colors.Reset)

	// Run git pull
	cmd := exec.Command("git", "pull")
	cmd.Dir = vcpkgRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	// Run bootstrap to ensure vcpkg binary is up to date
	fmt.Printf("%s Running bootstrap...%s\n", colors.Cyan, colors.Reset)

	var bootstrapCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		bootstrapCmd = exec.Command("cmd", "/c", "bootstrap-vcpkg.bat")
	} else {
		bootstrapCmd = exec.Command("./bootstrap-vcpkg.sh")
	}
	bootstrapCmd.Dir = vcpkgRoot
	bootstrapCmd.Stdout = os.Stdout
	bootstrapCmd.Stderr = os.Stderr

	if err := bootstrapCmd.Run(); err != nil {
		fmt.Printf("%s Bootstrap failed (vcpkg may still work): %v%s\n", colors.Yellow, err, colors.Reset)
	}

	fmt.Printf("%s vcpkg updated successfully!%s\n", colors.Green, colors.Reset)
	return nil
}
