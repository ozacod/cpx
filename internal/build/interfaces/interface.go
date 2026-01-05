// Package build provides build system abstractions for C++ projects.
//
// This package defines a common interface for different build systems
// (CMake, Bazel, Meson) and provides implementations for each.
package build

// DockerBuildOptions contains options for Docker-based builds.
type DockerBuildOptions struct {
	// ImageName is the Docker image to use for the build.
	ImageName string

	// ProjectRoot is the absolute path to the project root.
	ProjectRoot string

	// OutputDir is the relative path for build artifacts.
	OutputDir string

	// BuildType is the build type (Debug, Release, etc.).
	BuildType string

	// Optimization is the optimization level.
	Optimization string

	// CMakeArgs are additional CMake arguments.
	CMakeArgs []string

	// BuildArgs are additional build arguments.
	BuildArgs []string

	// MesonArgs are additional Meson arguments.
	MesonArgs []string

	// Jobs is the number of parallel jobs.
	Jobs int

	// Env contains environment variables for the build.
	Env map[string]string

	// ExecuteAfterBuild runs the executable after building.
	ExecuteAfterBuild bool

	// RunTests runs tests after building.
	RunTests bool

	// RunBenchmarks runs benchmarks after building.
	RunBenchmarks bool

	// Platform is the Docker platform (e.g., linux/amd64).
	Platform string

	// TargetName is the name of the toolchain/target.
	TargetName string

	// Verbose enables verbose output.
	Verbose bool
}

// DockerBuilder defines the interface for Docker-based builds.
type DockerBuilder interface {
	// RunDockerBuild runs a build inside a Docker container.
	RunDockerBuild(opts DockerBuildOptions) error
}

// DependencyInfo contains detailed information about a package
type DependencyInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Homepage     string   `json:"homepage"`
	License      string   `json:"license"`
	Dependencies []string `json:"dependencies"`
}

// BuildSystem defines the interface for all build system implementations.
// Each build system (CMake, Bazel, Meson) implements this interface to provide
// a unified way to build, test, run, and benchmark projects.
//
// The interface is composed of three smaller interfaces:
//   - Builder: core build operations
//   - DependencyManager: dependency management
//   - ProjectGenerator: project initialization
type BuildSystem interface {
	Builder
	DependencyManager
	ProjectGenerator
}

// Builder defines the core build operations interface.
type Builder interface {
	// Build compiles the project with the given options.
	Build(opts BuildOptions) error

	// Test runs the project's tests with the given options.
	Test(opts TestOptions) error

	// Run builds and runs the project's main executable.
	Run(opts RunOptions) error

	// Bench runs the project's benchmarks.
	Bench(opts BenchOptions) error

	// Clean removes build artifacts.
	Clean(opts CleanOptions) error

	// ListTargets returns the list of build targets (executables, libraries).
	ListTargets() ([]string, error)

	// Name returns the name of the build system (e.g., "cmake", "bazel", "meson").
	Name() string
}

// DependencyManager defines the interface for dependency management operations.
type DependencyManager interface {
	// ListDependencies returns the list of dependencies in the project.
	ListDependencies() ([]Dependency, error)

	// SearchDependencies searches for available packages matching the query.
	SearchDependencies(query string) ([]Dependency, error)

	// DependencyInfo retrieves detailed information about a specific dependency.
	DependencyInfo(name string) (*DependencyInfo, error)

	// AddDependency adds a dependency to the project.
	AddDependency(name string, version string) error

	// RemoveDependency removes a dependency from the project.
	RemoveDependency(name string) error

	// Update checks for outdated dependencies.
	Update() error

	// Upgrade upgrades dependencies to newer versions.
	Upgrade() error
}

// ProjectGenerator defines the interface for project initialization operations.
type ProjectGenerator interface {
	// GenerateGitignore generates the .gitignore file.
	GenerateGitignore(projectPath string) error

	// GenerateBuildSrc generates the build files for source code (core project files).
	GenerateBuildSrc(projectPath string, config InitConfig) error

	// GenerateBuildTest generates the build files for tests.
	GenerateBuildTest(projectPath string, config InitConfig) error

	// GenerateBuildBench generates the build files for benchmarks.
	GenerateBuildBench(projectPath string, config InitConfig) error

	// GenerateReadme generates the README.md content for the project.
	GenerateReadme(config InitConfig) string
}

// InitConfig contains configuration for initializing a new project.
type InitConfig struct {
	Name          string
	Version       string
	IsLibrary     bool
	CppStandard   int
	TestFramework string
	Benchmark     string
}

// Dependency represents a project dependency.
type Dependency struct {
	Name        string
	Version     string
	Description string
}

// BuildOptions contains options for building a project.
type BuildOptions struct {
	// Release indicates whether to build in release mode.
	Release bool

	// OptLevel overrides the optimization level (0, 1, 2, 3, s, fast).
	OptLevel string

	// Sanitizer specifies the sanitizer to use (asan, tsan, msan, ubsan).
	Sanitizer string

	// Target specifies a specific build target (optional).
	Target string

	// Jobs specifies the number of parallel jobs (0 = auto).
	Jobs int

	// Clean indicates whether to clean before building.
	Clean bool

	// Toolchain specifies a custom toolchain to use.
	Toolchain string

	// EnableTesting enables building test targets (-DENABLE_TESTING=ON).
	EnableTesting bool

	// EnableBenchmarks enables building benchmark targets (-DENABLE_BENCHMARKS=ON).
	EnableBenchmarks bool

	// OutputMode controls build output verbosity (UI, Verbose, or Quiet)
	OutputMode OutputMode
}

// OutputMode represents the build output verbosity mode
type OutputMode int

const (
	OutputModeUI      OutputMode = iota // Default: show progress UI
	OutputModeVerbose                   // Show full build output
	OutputModeQuiet                     // Minimal output
)

// TestOptions contains options for running tests.
type TestOptions struct {
	// Verbose enables verbose test output.
	Verbose bool

	// Filter filters tests by name pattern.
	Filter string

	// Toolchain specifies a custom toolchain to use.
	Toolchain string
}

// RunOptions contains options for running the project.
type RunOptions struct {
	// Release indicates whether to build in release mode before running.
	Release bool

	// OptLevel overrides the optimization level.
	OptLevel string

	// Sanitizer specifies the sanitizer to use.
	Sanitizer string

	// Target specifies the executable target to run.
	Target string

	// Args are arguments passed to the executable.
	Args []string

	// Verbose enables verbose output.
	Verbose bool

	// Toolchain specifies a custom toolchain to use.
	Toolchain string
}

// BenchOptions contains options for running benchmarks.
type BenchOptions struct {
	// Verbose enables verbose benchmark output.
	Verbose bool

	// Target specifies a specific benchmark target.
	Target string

	// Toolchain specifies a custom toolchain to use.
	Toolchain string
}

// CleanOptions contains options for cleaning build artifacts.
type CleanOptions struct {
	// All indicates whether to remove all build artifacts including caches.
	All bool

	// Verbose enables verbose output during cleaning.
	Verbose bool
}

// BuildResult contains the result of a build operation.
type BuildResult struct {
	// Success indicates whether the build succeeded.
	Success bool

	// Duration is the time taken for the build.
	Duration string

	// ArtifactDir is the directory containing build artifacts.
	ArtifactDir string

	// Executables is a list of executables produced by the build.
	Executables []string
}

// TestResult contains the result of a test run.
type TestResult struct {
	// Success indicates whether all tests passed.
	Success bool

	// Passed is the number of tests that passed.
	Passed int

	// Failed is the number of tests that failed.
	Failed int

	// Skipped is the number of tests that were skipped.
	Skipped int

	// Duration is the total time taken for the tests.
	Duration string
}

// GetOutputDir returns the appropriate output directory based on build options.
func GetOutputDir(release bool, optLevel, sanitizer string) string {
	outDirName := "debug"
	if optLevel != "" {
		outDirName = "O" + optLevel
	} else if release {
		outDirName = "release"
	}
	if sanitizer != "" {
		outDirName += "-" + sanitizer
	}
	return outDirName
}
