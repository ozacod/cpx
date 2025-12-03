package errors

import (
	"errors"
	"fmt"
)

// Error types for better error handling and user feedback

// ConfigError represents configuration-related errors
type ConfigError struct {
	Field   string
	Message string
	Hint    string
}

func (e *ConfigError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("config error: %s - %s\nHint: %s", e.Field, e.Message, e.Hint)
	}
	return fmt.Sprintf("config error: %s - %s", e.Field, e.Message)
}

// NewConfigError creates a new config error
func NewConfigError(field, message, hint string) *ConfigError {
	return &ConfigError{Field: field, Message: message, Hint: hint}
}

// BuildError represents build-related errors
type BuildError struct {
	Phase   string // configure, compile, link
	Message string
	Cause   error
}

func (e *BuildError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("build error [%s]: %s\nCaused by: %v", e.Phase, e.Message, e.Cause)
	}
	return fmt.Sprintf("build error [%s]: %s", e.Phase, e.Message)
}

func (e *BuildError) Unwrap() error {
	return e.Cause
}

// NewBuildError creates a new build error
func NewBuildError(phase, message string, cause error) *BuildError {
	return &BuildError{Phase: phase, Message: message, Cause: cause}
}

// DependencyError represents dependency-related errors
type DependencyError struct {
	Package string
	Message string
	Hint    string
}

func (e *DependencyError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("dependency error: %s - %s\nHint: %s", e.Package, e.Message, e.Hint)
	}
	return fmt.Sprintf("dependency error: %s - %s", e.Package, e.Message)
}

// NewDependencyError creates a new dependency error
func NewDependencyError(pkg, message, hint string) *DependencyError {
	return &DependencyError{Package: pkg, Message: message, Hint: hint}
}

// ToolError represents external tool-related errors
type ToolError struct {
	Tool       string
	Message    string
	InstallCmd string
}

func (e *ToolError) Error() string {
	if e.InstallCmd != "" {
		return fmt.Sprintf("%s: %s\nInstall with: %s", e.Tool, e.Message, e.InstallCmd)
	}
	return fmt.Sprintf("%s: %s", e.Tool, e.Message)
}

// NewToolError creates a new tool error
func NewToolError(tool, message, installCmd string) *ToolError {
	return &ToolError{Tool: tool, Message: message, InstallCmd: installCmd}
}

// Common errors
var (
	ErrNotInProject       = errors.New("not in a cpx project directory (no CMakeLists.txt found)")
	ErrNoVcpkgRoot        = errors.New("vcpkg_root not configured. Run: cpx config set-vcpkg-root <path>")
	ErrNoGitRepo          = errors.New("not in a git repository. Run: git init")
	ErrBuildNotConfigured = errors.New("project not configured. Run: cpx build first")
)

// IsConfigError checks if error is a config error
func IsConfigError(err error) bool {
	var configErr *ConfigError
	return errors.As(err, &configErr)
}

// IsBuildError checks if error is a build error
func IsBuildError(err error) bool {
	var buildErr *BuildError
	return errors.As(err, &buildErr)
}

// IsDependencyError checks if error is a dependency error
func IsDependencyError(err error) bool {
	var depErr *DependencyError
	return errors.As(err, &depErr)
}

// IsToolError checks if error is a tool error
func IsToolError(err error) bool {
	var toolErr *ToolError
	return errors.As(err, &toolErr)
}
