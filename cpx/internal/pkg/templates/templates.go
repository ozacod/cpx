package templates

import (
	"fmt"
	"strings"
	"unicode"
)

func safeIdent(name string) string {
	if name == "" {
		return "project"
	}
	var b strings.Builder
	for i, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			if i == 0 && unicode.IsDigit(r) {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "project"
	}
	return b.String()
}

func safeIdentUpper(name string) string {
	return strings.ToUpper(safeIdent(name))
}

func safeIdentTitle(name string) string {
	id := safeIdent(name)
	if id == "" {
		return "Project"
	}
	return strings.ToUpper(id[:1]) + id[1:]
}

// ============================================================================
// C++ SOURCE TEMPLATES
// ============================================================================

// generateVersionHpp generates version.hpp directly from project name and version
func GenerateVersionHpp(projectName, projectVersion string) string {
	if projectVersion == "" {
		projectVersion = "1.0.0"
	}
	safeNameUpper := safeIdentUpper(projectName)

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
	safeName := safeIdent(projectName)
	return fmt.Sprintf(`#include <%s/%s.hpp>
#include <iostream>

int main() {
    %s::greet();
    return 0;
}
`, projectName, projectName, safeName)
}

func GenerateLibHeader(projectName string) string {
	safeName := safeIdent(projectName)
	guard := safeIdentUpper(projectName) + "_HPP"
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
	safeName := safeIdent(projectName)

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
	safeName := safeIdent(projectName)
	safeNameTitle := safeIdentTitle(projectName)
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
		sb.WriteString(fmt.Sprintf(`# Library
add_library(%s
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
