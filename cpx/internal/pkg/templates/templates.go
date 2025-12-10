package templates

import (
	"fmt"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/naming"
)

// ============================================================================
// C++ SOURCE TEMPLATES
// ============================================================================

// generateVersionHpp generates version.hpp directly from project name and version
func GenerateVersionHpp(projectName, projectVersion string) string {
	if projectVersion == "" {
		projectVersion = "1.0.0"
	}
	safeNameUpper := naming.SafeIdentUpper(projectName)

	// Parse version components
	parts := strings.Split(projectVersion, ".")
	major := "0"
	minor := "0"
	patch := "0"
	if len(parts) > 0 {
		major = parts[0]
	}
	if len(parts) > 1 {
		minor = parts[1]
	}
	if len(parts) > 2 {
		patch = parts[2]
	}

	guard := safeNameUpper + "_VERSION_H_"

	return fmt.Sprintf(`#ifndef %s
#define %s

#define %s_VERSION "%s"
#define %s_MAJOR_VERSION %s
#define %s_MINOR_VERSION %s
#define %s_PATCH_VERSION %s

#endif  // %s
`, guard, guard, safeNameUpper, projectVersion, safeNameUpper, major, safeNameUpper, minor, safeNameUpper, patch, guard)
}

func GenerateMainCpp(projectName string) string {
	safeName := naming.SafeIdent(projectName)
	return fmt.Sprintf(`#include <%s/%s.hpp>
#include <iostream>

int main() {
    %s::greet();
    return 0;
}
`, projectName, projectName, safeName)
}

func GenerateLibHeader(projectName string) string {
	safeName := naming.SafeIdent(projectName)
	guard := naming.SafeIdentUpper(projectName) + "_HPP"
	return fmt.Sprintf(`#ifndef %s
#define %s

#include <string>

namespace %s {

/**
 * @brief Greet function
 */
void greet();

/**
 * @brief Get the library version
 * @return Version string
 */
std::string version();

}  // namespace %s

#endif  // %s
`, guard, guard, safeName, safeName, guard)
}

func GenerateLibSource(projectName string) string {
	safeName := naming.SafeIdent(projectName)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("#include <%s/%s.hpp>\n", projectName, projectName))
	sb.WriteString("#include <iostream>\n\n")
	sb.WriteString(fmt.Sprintf("namespace %s {\n\n", safeName))
	sb.WriteString(fmt.Sprintf(`void greet() {
    std::cout << "Hello from %s!" << std::endl;
}

std::string version() {
    return "1.0.0";
}

}  // namespace `+safeName+`
`, projectName))

	return sb.String()
}

func GenerateTestMain(projectName string, testingFramework string) string {
	safeName := naming.SafeIdent(projectName)
	safeNameTitle := naming.SafeIdentTitle(projectName)
	hasGtest := testingFramework == "googletest"
	hasCatch2 := testingFramework == "catch2"
	hasDoctest := testingFramework == "doctest"

	if hasGtest {
		return fmt.Sprintf(`#include <gtest/gtest.h>
#include <%s/%s.hpp>

TEST(%sTest, VersionTest) {
    EXPECT_EQ(%s::version(), "1.0.0");
}

TEST(%sTest, GreetTest) {
    // Should not throw
    EXPECT_NO_THROW(%s::greet());
}
`, projectName, projectName, safeNameTitle, safeName, safeNameTitle, safeName)
	} else if hasCatch2 {
		return fmt.Sprintf(`#include <catch2/catch_test_macros.hpp>
#include <%s/%s.hpp>

TEST_CASE("%s::version returns correct version", "[version]") {
    REQUIRE(%s::version() == "1.0.0");
}

TEST_CASE("%s::greet does not throw", "[greet]") {
    REQUIRE_NOTHROW(%s::greet());
}
`, projectName, projectName, safeName, safeName, safeName, safeName)
	} else if hasDoctest {
		return fmt.Sprintf(`#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include <doctest/doctest.h>
#include <%s/%s.hpp>

TEST_CASE("testing version") {
    CHECK(%s::version() == "1.0.0");
}

TEST_CASE("testing greet") {
    CHECK_NOTHROW(%s::greet());
}
`, projectName, projectName, safeName, safeName)
	} else {
		return fmt.Sprintf(`// Basic test file - add a test framework for better testing support
#include <%s/%s.hpp>
#include <cassert>
#include <iostream>

int main() {
    assert(%s::version() == "1.0.0");
    %s::greet();
    std::cout << "All tests passed!" << std::endl;
    return 0;
}
`, projectName, projectName, safeName, safeName)
	}
}

// ============================================================================
// CMAKE TEMPLATES
// ============================================================================

func GenerateVcpkgCMakeLists(projectName string, cppStandard int, isExe bool, includeTests bool, benchmarkFramework string, includeBench bool, projectVersion string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`cmake_minimum_required(VERSION 3.20)
project(%s VERSION %s LANGUAGES CXX)

# Set C++ standard
set(CMAKE_CXX_STANDARD %d)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Export compile commands for IDE support
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

`, projectName, projectVersion, cppStandard))

	if isExe {
		sb.WriteString(fmt.Sprintf(`# Executable
add_executable(%s
    src/main.cpp
    src/%s.cpp
)

target_include_directories(%s
    PRIVATE
        $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
)

`, projectName, projectName, projectName))
	} else {
		sb.WriteString(fmt.Sprintf(`# Library (static by default)
add_library(%s STATIC
    src/%s.cpp
)

target_include_directories(%s
    PUBLIC
        $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
        $<INSTALL_INTERFACE:include>
)

`, projectName, projectName, projectName))
	}

	if includeTests {
		sb.WriteString(`# Testing
enable_testing()
add_subdirectory(tests)
`)
	}

	if includeBench {
		sb.WriteString(`
# Benchmarks
add_subdirectory(bench)
`)
	}

	return sb.String()
}

// generateCMakePresets generates CMakePresets.json
// Assumes VCPKG_ROOT environment variable is set
func GenerateCMakePresets() string {
	return `{
  "version": 2,
  "configurePresets": [
    {
      "name": "default",
      "generator": "Ninja",
      "binaryDir": "${sourceDir}/build",
      "environment": {
        "VCPKG_DISABLE_REGISTRY_UPDATE": "1"
      },
      "cacheVariables": {
        "CMAKE_TOOLCHAIN_FILE": "$env{VCPKG_ROOT}/scripts/buildsystems/vcpkg.cmake"
      }
    }
  ]
}
`
}

func GenerateTestCMake(projectName string, testingFramework string) string {
	hasGtest := testingFramework == "googletest"
	hasCatch2 := testingFramework == "catch2"
	hasDoctest := testingFramework == "doctest"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`# Test configuration for %s

add_executable(%s_tests
    test_main.cpp
    ${CMAKE_CURRENT_SOURCE_DIR}/../src/%s.cpp
)

target_include_directories(%s_tests
    PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/../include
)

`, projectName, projectName, projectName, projectName))

	// Use FetchContent for testing frameworks
	if hasGtest {
		sb.WriteString(`# Fetch googletest
include(FetchContent)
FetchContent_Declare(
    googletest
    GIT_REPOSITORY https://github.com/google/googletest.git
    GIT_TAG v1.14.0
)
set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(googletest)

`)
		sb.WriteString(fmt.Sprintf("target_link_libraries(%s_tests PRIVATE gtest gtest_main gmock)\n\n", projectName))
		sb.WriteString("include(GoogleTest)\n")
		sb.WriteString(fmt.Sprintf("gtest_discover_tests(%s_tests)\n", projectName))
	} else if hasCatch2 {
		sb.WriteString(`# Fetch Catch2
include(FetchContent)
FetchContent_Declare(
    Catch2
    GIT_REPOSITORY https://github.com/catchorg/Catch2.git
    GIT_TAG v3.5.2
)
FetchContent_MakeAvailable(Catch2)

`)
		sb.WriteString(fmt.Sprintf("target_link_libraries(%s_tests PRIVATE Catch2::Catch2WithMain)\n\n", projectName))
		sb.WriteString("include(CTest)\n")
		sb.WriteString("include(Catch)\n")
		sb.WriteString(fmt.Sprintf("catch_discover_tests(%s_tests)\n", projectName))
	} else if hasDoctest {
		sb.WriteString(`# Fetch doctest
include(FetchContent)
FetchContent_Declare(
    doctest
    GIT_REPOSITORY https://github.com/doctest/doctest.git
    GIT_TAG v2.4.12
)
FetchContent_MakeAvailable(doctest)

`)
		sb.WriteString(fmt.Sprintf("target_link_libraries(%s_tests PRIVATE doctest::doctest)\n\n", projectName))
		sb.WriteString("include(CTest)\n")
		sb.WriteString(fmt.Sprintf("add_test(NAME %s_tests COMMAND %s_tests)\n", projectName, projectName))
	} else {
		sb.WriteString(fmt.Sprintf("add_test(NAME %s_tests COMMAND %s_tests)\n", projectName, projectName))
	}

	return sb.String()
}

// GenerateBenchCMake generates bench/CMakeLists.txt with FetchContent for benchmark frameworks
func GenerateBenchCMake(projectName string, benchmarkFramework string) string {
	hasGoogleBench := false
	hasCatch2Bench := false
	hasNanoBench := false

	// Check benchmarkFramework parameter
	switch strings.ToLower(benchmarkFramework) {
	case "google-benchmark":
		hasGoogleBench = true
	case "catch2-benchmark":
		hasCatch2Bench = true
	case "nanobench":
		hasNanoBench = true
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`# Benchmark configuration for %s

add_executable(%s_bench
    bench_main.cpp
    ${CMAKE_CURRENT_SOURCE_DIR}/../src/%s.cpp
)

target_include_directories(%s_bench
    PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/../include
)

`, projectName, projectName, projectName, projectName))

	// Use FetchContent for benchmark frameworks
	if hasGoogleBench {
		sb.WriteString(`# Fetch Google Benchmark
include(FetchContent)
FetchContent_Declare(
    benchmark
    GIT_REPOSITORY https://github.com/google/benchmark.git
    GIT_TAG v1.8.3
)
set(BENCHMARK_ENABLE_TESTING OFF CACHE BOOL "" FORCE)
set(BENCHMARK_ENABLE_INSTALL OFF CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(benchmark)

`)
		sb.WriteString(fmt.Sprintf("target_link_libraries(%s_bench PRIVATE benchmark::benchmark benchmark::benchmark_main)\n", projectName))
	} else if hasCatch2Bench {
		sb.WriteString(`# Fetch Catch2 for benchmarking
include(FetchContent)
FetchContent_Declare(
    Catch2
    GIT_REPOSITORY https://github.com/catchorg/Catch2.git
    GIT_TAG v3.5.2
)
FetchContent_MakeAvailable(Catch2)

`)
		sb.WriteString(fmt.Sprintf("target_link_libraries(%s_bench PRIVATE Catch2::Catch2WithMain)\n", projectName))
	} else if hasNanoBench {
		sb.WriteString(`# Fetch nanobench
include(FetchContent)
FetchContent_Declare(
    nanobench
    GIT_REPOSITORY https://github.com/martinus/nanobench.git
    GIT_TAG v4.3.11
)
FetchContent_MakeAvailable(nanobench)

`)
		sb.WriteString(fmt.Sprintf("target_link_libraries(%s_bench PRIVATE nanobench)\n", projectName))
	}

	return sb.String()
}

// ============================================================================
// CONFIGURATION TEMPLATES
// ============================================================================

func GenerateGitignore() string {
	return `# Build directories
build/
build-*/
build-docker-*/
out/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# Compiled files
*.o
*.obj
*.a
*.lib
*.so
*.dylib
*.dll

# CMake
CMakeFiles/
CMakeCache.txt
cmake_install.cmake
Makefile
compile_commands.json

# Testing
Testing/
test_results/

# Package
*.zip
*.tar.gz

# vcpkg cache (Docker builds)
.vcpkg_cache/
`
}

func GenerateClangFormat(style string) string {
	if style == "" {
		style = "Google"
	}

	// Common clang-format configurations
	baseConfig := `Language: Cpp
BasedOnStyle: %s
IndentWidth: 2
ColumnLimit: 100
AllowShortFunctionsOnASingleLine: Inline
AllowShortIfStatementsOnASingleLine: true
AllowShortLoopsOnASingleLine: true
BreakBeforeBraces: Attach
IndentCaseLabels: true
`

	switch style {
	case "Google":
		return fmt.Sprintf(baseConfig, "Google")
	case "LLVM":
		return fmt.Sprintf(baseConfig, "LLVM")
	case "Chromium":
		return fmt.Sprintf(baseConfig, "Chromium")
	case "Mozilla":
		return fmt.Sprintf(baseConfig, "Mozilla")
	case "WebKit":
		return fmt.Sprintf(baseConfig, "WebKit")
	case "Microsoft":
		return fmt.Sprintf(baseConfig, "Microsoft")
	default:
		return fmt.Sprintf(baseConfig, "Google")
	}
}

// generateCpxCI generates a cpx.ci file with empty targets
func GenerateCpxCI() string {
	return `# cpx.ci - Cross-compilation configuration
# This file defines which Docker images to use for building your project
# Add targets to build for different platforms

# List of targets to build
targets: []

# Build configuration
build:
  # CMake build type (Debug, Release, RelWithDebInfo, MinSizeRel)
  type: Release

  # Optimization level (0, 1, 2, 3, s, fast)
  optimization: 2

  # Number of parallel jobs (0 = auto)
  jobs: 0

  # Additional CMake arguments
  cmake_args: []

  # Additional build arguments
  build_args: []

# Output directory for artifacts
output: out
`
}

// ============================================================================
// DOCUMENTATION TEMPLATES
// ============================================================================

// generateVcpkgReadme generates README with vcpkg instructions
func GenerateVcpkgReadme(projectName string, cppStandard int, isLib bool) string {
	codeBlock := "```"
	if isLib {
		return fmt.Sprintf(`# %s

A C++ library using vcpkg for dependency management.

## Requirements

- CMake 3.20 or higher
- C++%d compatible compiler
- vcpkg

## Building

%sbash
cmake --preset=default
cmake --build build
%s

## Installation

%sbash
cd build
cmake --install . --prefix /usr/local
%s

## Usage

%scmake
find_package(%s REQUIRED)
target_link_libraries(your_target PRIVATE %s)
%s

## Testing

%sbash
cd build
ctest --output-on-failure
%s

## License

MIT
`, projectName, cppStandard, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock, projectName, projectName, codeBlock, codeBlock, codeBlock)
	} else {
		return fmt.Sprintf(`# %s

A C++ project using vcpkg for dependency management.

## Requirements

- CMake 3.20 or higher
- C++%d compatible compiler
- vcpkg

## Building

%sbash
cmake --preset=default
cmake --build build
%s

## Running

%sbash
./build/%s
%s

## Testing

%sbash
cd build
ctest --output-on-failure
%s

## License

MIT
`, projectName, cppStandard, codeBlock, codeBlock, codeBlock, projectName, codeBlock, codeBlock, codeBlock)
	}
}

// ============================================================================
// BAZEL TEMPLATES
// ============================================================================

// GenerateModuleBazel generates MODULE.bazel content for a Bazel project
func GenerateModuleBazel(projectName, version, testFramework, benchmarkFramework string) string {
	if version == "" {
		version = "0.1.0"
	}

	// Base module definition
	content := fmt.Sprintf(`module(
    name = "%s",
    version = "%s",
)

bazel_dep(name = "rules_cc", version = "0.1.1")
`, projectName, version)

	// Add test framework dependency
	switch testFramework {
	case "googletest":
		content += `bazel_dep(name = "googletest", version = "1.15.2")
`
	case "catch2":
		content += `bazel_dep(name = "catch2", version = "3.7.1")
`
	case "doctest":
		content += `bazel_dep(name = "doctest", version = "2.4.11")
`
	}

	// Add benchmark framework dependency
	switch benchmarkFramework {
	case "google-benchmark", "googlebenchmark":
		content += `bazel_dep(name = "google_benchmark", version = "1.9.1")
`
	case "nanobench":
		content += `bazel_dep(name = "nanobench", version = "4.3.11")
`
	case "catch2-benchmark":
		// Catch2 already includes benchmark support
		if testFramework != "catch2" {
			content += `bazel_dep(name = "catch2", version = "3.7.1")
`
		}
	}

	return content
}

// GenerateBuildBazelRoot generates root BUILD.bazel (empty or just aliases)
func GenerateBuildBazelRoot(projectName string, isExe bool) string {
	if isExe {
		return fmt.Sprintf(`# Root BUILD.bazel - aliases for convenience
# Main targets are in //src:

# Alias to main binary
alias(
    name = "%s",
    actual = "//src:%s",
)

# Alias to library
alias(
    name = "%s_lib",
    actual = "//src:%s_lib",
    visibility = ["//visibility:public"],
)
`, projectName, projectName, projectName, projectName)
	}
	return fmt.Sprintf(`# Root BUILD.bazel - aliases for convenience

# Alias to main library
alias(
    name = "%s",
    actual = "//src:%s",
    visibility = ["//visibility:public"],
)
`, projectName, projectName)
}

// GenerateBuildBazelSrc generates src/BUILD.bazel
func GenerateBuildBazelSrc(projectName string, isExe bool) string {
	if isExe {
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_binary", "cc_library")

# Core library
cc_library(
    name = "%s_lib",
    srcs = ["%s.cpp"],
    deps = ["//include:%s_headers"],
    visibility = ["//visibility:public"],
)

# Main executable
cc_binary(
    name = "%s",
    srcs = ["main.cpp"],
    deps = [":%s_lib"],
    visibility = ["//visibility:public"],
)
`, projectName, projectName, projectName, projectName, projectName)
	}
	return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_library")

# Core library
cc_library(
    name = "%s",
    srcs = ["%s.cpp"],
    deps = ["//include:%s_headers"],
    visibility = ["//visibility:public"],
)
`, projectName, projectName, projectName)
}

// GenerateBuildBazelInclude generates include/BUILD.bazel
func GenerateBuildBazelInclude(projectName string) string {
	return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_library")

# Header-only library (public headers)
cc_library(
    name = "%s_headers",
    hdrs = glob(["%s/*.hpp"]),
    includes = ["."],
    visibility = ["//visibility:public"],
)
`, projectName, projectName)
}

// GenerateBuildBazelTests generates tests/BUILD.bazel
func GenerateBuildBazelTests(projectName string, testFramework string) string {
	switch testFramework {
	case "googletest":
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_test")

cc_test(
    name = "%s_test",
    srcs = ["test_main.cpp"],
    deps = [
        "//src:%s_lib",
        "@googletest//:gtest_main",
    ],
)
`, projectName, projectName)

	case "catch2":
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_test")

cc_test(
    name = "%s_test",
    srcs = ["test_main.cpp"],
    deps = [
        "//src:%s_lib",
        "@catch2//:catch2_main",
    ],
)
`, projectName, projectName)

	case "doctest":
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_test")

cc_test(
    name = "%s_test",
    srcs = ["test_main.cpp"],
    deps = [
        "//src:%s_lib",
        "@doctest//:doctest",
    ],
)
`, projectName, projectName)

	default:
		// Basic test without framework
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_test")

cc_test(
    name = "%s_test",
    srcs = ["test_main.cpp"],
    deps = [
        "//src:%s_lib",
    ],
)
`, projectName, projectName)
	}
}

// GenerateBuildBazelBench generates bench/BUILD.bazel
func GenerateBuildBazelBench(projectName, benchmarkFramework string) string {
	switch benchmarkFramework {
	case "google-benchmark", "googlebenchmark":
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_binary")

cc_binary(
    name = "%s_bench",
    srcs = ["bench_main.cpp"],
    deps = [
        "//src:%s_lib",
        "@google_benchmark//:benchmark_main",
    ],
)
`, projectName, projectName)

	case "nanobench":
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_binary")

cc_binary(
    name = "%s_bench",
    srcs = ["bench_main.cpp"],
    deps = [
        "//src:%s_lib",
        "@nanobench//:nanobench",
    ],
)
`, projectName, projectName)

	case "catch2-benchmark":
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_binary")

cc_binary(
    name = "%s_bench",
    srcs = ["bench_main.cpp"],
    deps = [
        "//src:%s_lib",
        "@catch2//:catch2_main",
    ],
)
`, projectName, projectName)

	default:
		// Default to google benchmark
		return fmt.Sprintf(`load("@rules_cc//cc:defs.bzl", "cc_binary")

cc_binary(
    name = "%s_bench",
    srcs = ["bench_main.cpp"],
    deps = [
        "//src:%s_lib",
    ],
)
`, projectName, projectName)
	}
}

// GenerateBazelrc generates .bazelrc with common settings
func GenerateBazelrc(cppStandard int) string {
	return fmt.Sprintf(`# C++ standard
build --cxxopt=-std=c++%d

# Hide bazel symlinks (creates .bin, .out, etc.)
build --symlink_prefix=.

# Enable optimizations for release builds
build:release --compilation_mode=opt

# Debug build configuration
build:debug --compilation_mode=dbg
build:debug --cxxopt=-g

# Enable colored output
build --color=yes

# Show test output
test --test_output=errors
`, cppStandard)
}

// GenerateBazelignore generates .bazelignore file
func GenerateBazelignore() string {
	return `# Ignore build output directory
build

# Ignore git
.git

# Ignore IDE directories
.idea
.vscode

# Ignore cpx cache
.cache
`
}

// GenerateBazelGitignore generates .gitignore for Bazel projects
func GenerateBazelGitignore() string {
	return `# Bazel
bazel-*
.bazel-*

# Build output
build/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# Cache
.cache/

# Compiled files
*.o
*.obj
*.a
*.lib
*.so
*.dylib
*.dll
`
}

// GenerateBazelReadme generates README with Bazel instructions
func GenerateBazelReadme(projectName string, cppStandard int, isLib bool) string {
	codeBlock := "```"
	if isLib {
		return fmt.Sprintf(`# %s

A C++ library using Bazel for builds and dependency management.

## Requirements

- Bazel 7.0+ (Bzlmod support)
- C++%d compatible compiler

## Building

%sbash
cpx build
# or: bazel build //...
%s

## Usage

Add to your MODULE.bazel:

%sstarlark
bazel_dep(name = "%s", version = "0.1.0")
%s

Then in your BUILD.bazel:

%sstarlark
cc_binary(
    name = "your_app",
    srcs = ["main.cpp"],
    deps = ["@%s"],
)
%s

## Testing

%sbash
cpx test
# or: bazel test //...
%s

## License

MIT
`, projectName, cppStandard, codeBlock, codeBlock, codeBlock, projectName, codeBlock, codeBlock, projectName, codeBlock, codeBlock, codeBlock)
	}
	return fmt.Sprintf(`# %s

A C++ project using Bazel for builds and dependency management.

## Requirements

- Bazel 7.0+ (Bzlmod support)
- C++%d compatible compiler

## Building

%sbash
cpx build
# or: bazel build //...
%s

## Running

%sbash
cpx run
# or: bazel run //:%s
%s

## Testing

%sbash
cpx test
# or: bazel test //...
%s

## Adding Dependencies

%sbash
cpx add abseil-cpp
%s

## License

MIT
`, projectName, cppStandard, codeBlock, codeBlock, codeBlock, projectName, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock)
}

// ============================================================================
// MESON TEMPLATES
// ============================================================================

// GenerateMesonBuildRoot generates root meson.build
func GenerateMesonBuildRoot(projectName string, isExe bool, cppStandard int, testFramework, benchmarkFramework string) string {
	targetType := "executable"
	if !isExe {
		targetType = "library"
	}

	// Build subdir includes
	subdirs := "subdir('src')\n"
	if testFramework != "" && testFramework != "none" {
		subdirs += "subdir('tests')\n"
	}
	if benchmarkFramework != "" && benchmarkFramework != "none" {
		subdirs += "subdir('bench')\n"
	}

	return fmt.Sprintf(`project('%s', 'cpp',
  version : '0.1.0',
  default_options : [
    'cpp_std=c++%d',
    'warning_level=3',
    'buildtype=debugoptimized'
  ]
)

# Include directories
inc_dirs = include_directories('include')

# Subdirectories
%s
`, projectName, cppStandard, subdirs) + fmt.Sprintf(`
# Summary
summary({
  'Project': '%s',
  'Type': '%s',
  'C++ Standard': 'C++%d',
}, section: 'Configuration')
`, projectName, targetType, cppStandard)
}

// GenerateMesonBuildSrc generates src/meson.build
func GenerateMesonBuildSrc(projectName string, isExe bool) string {
	safeName := naming.SafeIdent(projectName)

	if isExe {
		return fmt.Sprintf(`# Source files
src_files = files(
  'main.cpp',
  '%s.cpp'
)

# Library (for linking by tests/benchmarks)
%s_lib = static_library('%s_lib',
  files('%s.cpp'),
  include_directories : inc_dirs,
  install : true
)

# Executable
%s_exe = executable('%s',
  src_files,
  include_directories : inc_dirs,
  install : true
)
`, projectName, safeName, safeName, projectName, safeName, projectName)
	}

	// Library only
	return fmt.Sprintf(`# Source files
src_files = files(
  '%s.cpp'
)

# Library
%s_lib = library('%s',
  src_files,
  include_directories : inc_dirs,
  install : true
)
`, projectName, safeName, projectName)
}

// GenerateMesonBuildTests generates tests/meson.build
func GenerateMesonBuildTests(projectName, testFramework string) string {
	safeName := naming.SafeIdent(projectName)

	var depLine string
	switch testFramework {
	case "googletest":
		depLine = "gtest_dep = dependency('gtest', fallback : ['gtest', 'gtest_main_dep'])"
	case "catch2":
		depLine = "catch2_dep = dependency('catch2-with-main', fallback : ['catch2', 'catch2_with_main_dep'])"
	case "doctest":
		depLine = "doctest_dep = dependency('doctest', fallback : ['doctest', 'doctest_dep'])"
	default:
		depLine = "# No test framework"
	}

	var depsArg string
	switch testFramework {
	case "googletest":
		depsArg = "gtest_dep"
	case "catch2":
		depsArg = "catch2_dep"
	case "doctest":
		depsArg = "doctest_dep"
	default:
		depsArg = ""
	}

	if depsArg != "" {
		depsArg = ",\n  dependencies : [" + depsArg + "]"
	}

	return fmt.Sprintf(`# Test dependencies
%s

# Test executable
test_exe = executable('%s_test',
  files('test_main.cpp'),
  include_directories : inc_dirs,
  link_with : %s_lib%s
)

# Register test
test('%s tests', test_exe)
`, depLine, projectName, safeName, depsArg, projectName)
}

// GenerateMesonBuildBench generates bench/meson.build
func GenerateMesonBuildBench(projectName, benchmarkFramework string) string {
	safeName := naming.SafeIdent(projectName)

	var depLine, depsArg string
	switch benchmarkFramework {
	case "google-benchmark":
		depLine = "benchmark_dep = dependency('benchmark', fallback : ['google-benchmark', 'google_benchmark_dep'])"
		depsArg = "benchmark_dep"
	case "nanobench":
		depLine = "# nanobench is header-only"
		depsArg = ""
	case "catch2-benchmark":
		depLine = "catch2_dep = dependency('catch2-with-main', fallback : ['catch2', 'catch2_with_main_dep'])"
		depsArg = "catch2_dep"
	default:
		depLine = "# No benchmark framework"
		depsArg = ""
	}

	if depsArg != "" {
		depsArg = ",\n  dependencies : [" + depsArg + "]"
	}

	return fmt.Sprintf(`# Benchmark dependencies
%s

# Benchmark executable
bench_exe = executable('%s_bench',
  files('bench_main.cpp'),
  include_directories : inc_dirs,
  link_with : %s_lib%s
)

# Run benchmark (not as a test, just build)
`, depLine, projectName, safeName, depsArg)
}

// GenerateMesonOptions generates meson.options
func GenerateMesonOptions() string {
	return `# Build options
option('enable_tests', type : 'boolean', value : true,
       description : 'Enable building tests')

option('enable_benchmarks', type : 'boolean', value : true,
       description : 'Enable building benchmarks')
`
}

// GenerateMesonGitignore generates .gitignore for Meson projects
func GenerateMesonGitignore() string {
	return `# Meson build directory
builddir/
build/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# Cache
.cache/

# Compiled files
*.o
*.obj
*.a
*.lib
*.so
*.dylib
*.dll
`
}

// GenerateMesonReadme generates README with Meson instructions
func GenerateMesonReadme(projectName string, cppStandard int, isLib bool) string {
	codeBlock := "```"
	if isLib {
		return fmt.Sprintf(`# %s

A C++ library using Meson for builds.

## Requirements

- C++%d compatible compiler
- Meson (>= 0.60.0)
- Ninja (recommended backend)

## Building

%sbash
# Configure
cpx build
# Or manually:
meson setup builddir
meson compile -C builddir
%s

## Testing

%sbash
cpx test
# Or manually:
meson test -C builddir
%s

## Adding Dependencies

%sbash
cpx add <package-name>
%s

This downloads wrap files to subprojects/ directory.

## License

MIT
`, projectName, cppStandard, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock)
	}

	// Executable project
	return fmt.Sprintf(`# %s

A C++ application using Meson for builds.

## Requirements

- C++%d compatible compiler
- Meson (>= 0.60.0)
- Ninja (recommended backend)

## Building

%sbash
cpx build
%s

## Running

%sbash
cpx run
%s

## Testing

%sbash
cpx test
%s

## Adding Dependencies

%sbash
cpx add <package-name>
%s

This downloads wrap files to subprojects/ directory.

## License

MIT
`, projectName, cppStandard, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock, codeBlock)
}

// ============================================================================
// Benchmark Source Templates
// ============================================================================

// BenchSources holds generated benchmark source files
type BenchSources struct {
	Main string
}

// GenerateBenchmarkSources generates benchmark source files based on framework
func GenerateBenchmarkSources(projectName, benchmarkFramework string) (*BenchSources, []string) {
	safeName := SafeIdent(projectName)

	switch benchmarkFramework {
	case "google-benchmark":
		return &BenchSources{Main: generateGoogleBenchMain(projectName, safeName)}, []string{"benchmark"}
	case "nanobench":
		return &BenchSources{Main: generateNanoBenchMain(projectName, safeName)}, []string{"nanobench"}
	case "catch2-benchmark":
		return &BenchSources{Main: generateCatch2BenchMain(projectName, safeName)}, []string{"catch2"}
	default:
		return nil, nil
	}
}

func generateGoogleBenchMain(projectName, safeName string) string {
	return fmt.Sprintf(`#include <benchmark/benchmark.h>
#include <%s/%s.hpp>

static void BM_version(benchmark::State& state) {
    for (auto _ : state) {
        benchmark::DoNotOptimize(%s::version());
    }
}

BENCHMARK(BM_version);

int main(int argc, char** argv) {
    benchmark::Initialize(&argc, argv);
    if (benchmark::ReportUnrecognizedArguments(argc, argv)) return 1;
    benchmark::RunSpecifiedBenchmarks();
}
`, projectName, projectName, safeName)
}

func generateNanoBenchMain(projectName, safeName string) string {
	return fmt.Sprintf(`#include <nanobench.h>
#include <%s/%s.hpp>
#include <iostream>

int main() {
    ankerl::nanobench::Bench bench;
    bench.run("version", [] {
        ankerl::nanobench::doNotOptimizeAway(%s::version());
    });
    return 0;
}
`, projectName, projectName, safeName)
}

func generateCatch2BenchMain(projectName, safeName string) string {
	return fmt.Sprintf(`#include <catch2/catch_all.hpp>
#include <%s/%s.hpp>

TEST_CASE("Benchmark version", "[benchmark]") {
    BENCHMARK("version") {
        return %s::version();
    };
}
`, projectName, projectName, safeName)
}

// SafeIdent converts a project name to a valid C++ identifier
func SafeIdent(name string) string {
	result := ""
	for i, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			result += string(c)
		} else if c >= '0' && c <= '9' {
			if i == 0 {
				result += "_"
			}
			result += string(c)
		} else if c == '-' || c == ' ' {
			result += "_"
		}
	}
	if result == "" {
		result = "project"
	}
	return result
}
