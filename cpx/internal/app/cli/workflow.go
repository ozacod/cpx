package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ozacod/cpx/pkg/config"
	"github.com/spf13/cobra"
)

// WorkflowCmd creates the workflow command for generating CI/CD workflows
func WorkflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Generate CI/CD workflow files",
		Long:  "Generate workflow files for various CI/CD platforms (GitHub Actions, GitLab CI).",
	}

	// Add github-actions subcommand
	githubCmd := &cobra.Command{
		Use:   "github-actions",
		Short: "Generate GitHub Actions workflow",
		Long:  "Generate a GitHub Actions workflow file for building with cpx ci.",
		RunE:  runGenerateGitHub,
	}
	cmd.AddCommand(githubCmd)

	// Add gitlab subcommand
	gitlabCmd := &cobra.Command{
		Use:   "gitlab",
		Short: "Generate GitLab CI configuration",
		Long:  "Generate a GitLab CI configuration file for building with cpx ci.",
		RunE:  runGenerateGitLab,
	}
	cmd.AddCommand(gitlabCmd)

	return cmd
}

func runGenerateGitHub(_ *cobra.Command, _ []string) error {
	if err := generateGitHubActionsWorkflow(); err != nil {
		return err
	}
	fmt.Printf("%s✓ Created GitHub Actions workflow: .github/workflows/ci.yml%s\n", Green, Reset)
	return nil
}

func runGenerateGitLab(_ *cobra.Command, _ []string) error {
	if err := generateGitLabCI(); err != nil {
		return err
	}
	fmt.Printf("%s✓ Created GitLab CI configuration: .gitlab-ci.yml%s\n", Green, Reset)
	return nil
}

func generateGitHubActionsWorkflow() error {
	// Get project root (look for cpx-ci.yaml or go up until we find it or reach root)
	projectRoot, err := findProjectRoot()
	if err != nil {
		// If we can't find project root, use current directory
		projectRoot, _ = os.Getwd()
	}

	// Try to load cpx-ci.yaml (optional - will create basic workflow if not found)
	ciConfigPath := filepath.Join(projectRoot, "cpx-ci.yaml")
	ciConfig, err := config.LoadToolchains(ciConfigPath)
	outputDir := "out"
	if err != nil {
		fmt.Printf("%s Warning: cpx-ci.yaml not found. Creating basic workflow.%s\n", Yellow, Reset)
		fmt.Printf("  Create cpx-ci.yaml to customize build targets and configuration.\n")
	} else {
		outputDir = ciConfig.Output
		if outputDir == "" {
			outputDir = filepath.Join(".bin", "ci")
		}
	}

	// Create .github/workflows directory in project root
	workflowsDir := filepath.Join(projectRoot, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create .github/workflows directory: %w", err)
	}

	workflowFile := filepath.Join(workflowsDir, "ci.yml")

	// Generate workflow content
	workflowContent := `name: CI

on:
  push:
    branches: [ main, master, develop ]
  pull_request:
    branches: [ main, master, develop ]

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Install cpx
        run: |
          curl -fsSL https://raw.githubusercontent.com/ozacod/cpx/main/install.sh | sh
          echo "$HOME/.local/bin" >> $GITHUB_PATH

      - name: Install Docker
        uses: docker/setup-buildx-action@v3

      - name: Run cpx ci
        run: cpx ci
`

	// Add artifact upload if output directory is specified
	if outputDir != "" {
		workflowContent += `
      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: build-artifacts
          path: ` + outputDir + `
`
	}

	if err := os.WriteFile(workflowFile, []byte(workflowContent), 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	return nil
}

func generateGitLabCI() error {
	// Get project root (look for cpx-ci.yaml or go up until we find it or reach root)
	projectRoot, err := findProjectRoot()
	if err != nil {
		// If we can't find project root, use current directory
		projectRoot, _ = os.Getwd()
	}

	// Try to load cpx-ci.yaml (optional - will create basic CI if not found)
	ciConfigPath := filepath.Join(projectRoot, "cpx-ci.yaml")
	ciConfig, err := config.LoadToolchains(ciConfigPath)
	outputDir := "out"
	if err != nil {
		fmt.Printf("%s Warning: cpx-ci.yaml not found. Creating basic CI configuration.%s\n", Yellow, Reset)
		fmt.Printf("  Create cpx-ci.yaml to customize build targets and configuration.\n")
	} else {
		outputDir = ciConfig.Output
		if outputDir == "" {
			outputDir = filepath.Join(".bin", "ci")
		}
	}

	gitlabCIFile := filepath.Join(projectRoot, ".gitlab-ci.yml")

	// Generate GitLab CI content
	gitlabCIContent := `image: golang:1.21

variables:
  CPX_VERSION: latest

before_script:
  - apt-get update && apt-get install -y curl docker.io
  - systemctl start docker || true
  - curl -fsSL https://raw.githubusercontent.com/ozacod/cpx/main/install.sh | sh
  - export PATH="$HOME/.local/bin:$PATH"

build:
  stage: build
  script:
    - cpx ci
`

	// Add artifacts if output directory is specified
	if outputDir != "" {
		gitlabCIContent += `  artifacts:
    paths:
      - ` + outputDir + `
    expire_in: 1 week
`
	}

	if err := os.WriteFile(gitlabCIFile, []byte(gitlabCIContent), 0644); err != nil {
		return fmt.Errorf("failed to write GitLab CI file: %w", err)
	}

	return nil
}
