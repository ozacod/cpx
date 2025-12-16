package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateModuleBazel(t *testing.T) {
	tests := []struct {
		name               string
		projectName        string
		version            string
		testFramework      string
		benchmarkFramework string
		shouldContain      []string
		shouldNotContain   []string
	}{
		{
			name:               "Basic project",
			projectName:        "myproject",
			version:            "1.0.0",
			testFramework:      "",
			benchmarkFramework: "",
			shouldContain:      []string{"module(", `name = "myproject"`, `version = "1.0.0"`, "rules_cc"},
			shouldNotContain:   []string{"googletest", "catch2", "google_benchmark"},
		},
		{
			name:               "With googletest",
			projectName:        "testproject",
			version:            "0.1.0",
			testFramework:      "googletest",
			benchmarkFramework: "",
			shouldContain:      []string{"googletest"},
			shouldNotContain:   []string{"catch2", "google_benchmark"},
		},
		{
			name:               "With catch2",
			projectName:        "catchproject",
			version:            "0.1.0",
			testFramework:      "catch2",
			benchmarkFramework: "",
			shouldContain:      []string{"catch2"},
			shouldNotContain:   []string{"googletest"},
		},
		{
			name:               "With google benchmark",
			projectName:        "benchproject",
			version:            "0.1.0",
			testFramework:      "",
			benchmarkFramework: "google-benchmark",
			shouldContain:      []string{"google_benchmark"},
			shouldNotContain:   []string{"googletest"},
		},
		{
			name:               "Default version",
			projectName:        "project",
			version:            "",
			testFramework:      "",
			benchmarkFramework: "",
			shouldContain:      []string{`version = "0.1.0"`},
			shouldNotContain:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateModuleBazel(tt.projectName, tt.version, tt.testFramework, tt.benchmarkFramework)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
			for _, s := range tt.shouldNotContain {
				assert.NotContains(t, result, s, "Expected NOT to contain: %s", s)
			}
		})
	}
}

func TestGenerateBuildBazelRoot(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		isExe         bool
		shouldContain []string
	}{
		{
			name:        "Executable project",
			projectName: "myapp",
			isExe:       true,
			shouldContain: []string{
				"alias(",
				`name = "myapp"`,
				`actual = "//src:myapp"`,
				`name = "myapp_lib"`,
			},
		},
		{
			name:        "Library project",
			projectName: "mylib",
			isExe:       false,
			shouldContain: []string{
				"alias(",
				`name = "mylib"`,
				`actual = "//src:mylib"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateBuildBazelRoot(tt.projectName, tt.isExe)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateBuildBazelTests(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		testFramework string
		shouldContain []string
	}{
		{
			name:          "GoogleTest",
			projectName:   "myproject",
			testFramework: "googletest",
			shouldContain: []string{"cc_test", "@googletest//:gtest_main", `name = "myproject_test"`},
		},
		{
			name:          "Catch2",
			projectName:   "myproject",
			testFramework: "catch2",
			shouldContain: []string{"cc_test", "@catch2//:catch2_main"},
		},
		{
			name:          "Doctest",
			projectName:   "myproject",
			testFramework: "doctest",
			shouldContain: []string{"cc_test", "@doctest//:doctest"},
		},
		{
			name:          "No framework",
			projectName:   "myproject",
			testFramework: "",
			shouldContain: []string{"cc_test", `"//src:myproject_lib"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateBuildBazelTests(tt.projectName, tt.testFramework)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateBazelrc(t *testing.T) {
	tests := []struct {
		name          string
		cppStandard   int
		shouldContain []string
	}{
		{
			name:          "C++17",
			cppStandard:   17,
			shouldContain: []string{"c++17", "--symlink_prefix=.", "build:release", "build:debug"},
		},
		{
			name:          "C++20",
			cppStandard:   20,
			shouldContain: []string{"c++20"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateBazelrc(tt.cppStandard)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateBenchmarkSources(t *testing.T) {
	tests := []struct {
		name               string
		projectName        string
		benchmarkFramework string
		expectedDeps       []string
		shouldBeNil        bool
	}{
		{
			name:               "Google Benchmark",
			projectName:        "myproject",
			benchmarkFramework: "google-benchmark",
			expectedDeps:       []string{"benchmark"},
			shouldBeNil:        false,
		},
		{
			name:               "Nanobench",
			projectName:        "myproject",
			benchmarkFramework: "nanobench",
			expectedDeps:       []string{"nanobench"},
			shouldBeNil:        false,
		},
		{
			name:               "Catch2 Benchmark",
			projectName:        "myproject",
			benchmarkFramework: "catch2-benchmark",
			expectedDeps:       []string{"catch2"},
			shouldBeNil:        false,
		},
		{
			name:               "Unknown framework",
			projectName:        "myproject",
			benchmarkFramework: "unknown",
			expectedDeps:       nil,
			shouldBeNil:        true,
		},
		{
			name:               "Empty framework",
			projectName:        "myproject",
			benchmarkFramework: "",
			expectedDeps:       nil,
			shouldBeNil:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources, deps := GenerateBenchmarkSources(tt.projectName, tt.benchmarkFramework)

			if tt.shouldBeNil {
				assert.Nil(t, sources)
				assert.Nil(t, deps)
			} else {
				assert.NotNil(t, sources)
				assert.NotEmpty(t, sources.Main)
				assert.Equal(t, tt.expectedDeps, deps)
				// Verify the benchmark includes the project header
				assert.Contains(t, sources.Main, tt.projectName)
			}
		})
	}
}

func TestSafeIdent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple name",
			input:    "myproject",
			expected: "myproject",
		},
		{
			name:     "Name with hyphens",
			input:    "my-project",
			expected: "my_project",
		},
		{
			name:     "Empty name",
			input:    "",
			expected: "project",
		},
		{
			name:     "Starting with digit",
			input:    "123abc",
			expected: "_123abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeIdent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateVersionHpp(t *testing.T) {
	result := GenerateVersionHpp("myproject", "1.2.3")

	// Should contain version components
	assert.True(t, strings.Contains(result, "1") && strings.Contains(result, "2") && strings.Contains(result, "3"))
	// Should contain project name in include guard
	assert.Contains(t, result, "MYPROJECT")
	// Should be valid C++ header
	assert.Contains(t, result, "#ifndef")
	assert.Contains(t, result, "#define")
	assert.Contains(t, result, "#endif")
}

func TestGenerateGitignore(t *testing.T) {
	result := GenerateGitignore()

	// Should ignore common build artifacts
	assert.Contains(t, result, "build")
	assert.Contains(t, result, ".cmake")
	// Should ignore IDE files
	assert.Contains(t, result, ".vscode")
	assert.Contains(t, result, ".idea")
}

func TestGenerateBazelGitignore(t *testing.T) {
	result := GenerateBazelGitignore()

	// Should ignore Bazel artifacts
	assert.Contains(t, result, "bazel-")
	assert.Contains(t, result, ".bazel-")
	// Should ignore build directory
	assert.Contains(t, result, "build")
}

// ============================================================================
// MESON TEMPLATE TESTS
// ============================================================================

func TestGenerateMesonBuildRoot(t *testing.T) {
	tests := []struct {
		name               string
		projectName        string
		isExe              bool
		cppStandard        int
		testFramework      string
		benchmarkFramework string
		shouldContain      []string
		shouldNotContain   []string
	}{
		{
			name:               "Executable with tests and bench",
			projectName:        "myapp",
			isExe:              true,
			cppStandard:        17,
			testFramework:      "googletest",
			benchmarkFramework: "google-benchmark",
			shouldContain: []string{
				"project('myapp', 'cpp'",
				"cpp_std=c++17",
				"subdir('src')",
				"subdir('tests')",
				"subdir('bench')",
				"inc_dirs = include_directories",
			},
		},
		{
			name:               "Library without tests",
			projectName:        "mylib",
			isExe:              false,
			cppStandard:        20,
			testFramework:      "",
			benchmarkFramework: "",
			shouldContain: []string{
				"project('mylib', 'cpp'",
				"cpp_std=c++20",
				"subdir('src')",
			},
			shouldNotContain: []string{
				"subdir('tests')",
				"subdir('bench')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateMesonBuildRoot(tt.projectName, tt.isExe, tt.cppStandard, tt.testFramework, tt.benchmarkFramework)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
			for _, s := range tt.shouldNotContain {
				assert.NotContains(t, result, s, "Expected NOT to contain: %s", s)
			}
		})
	}
}

func TestGenerateMesonBuildSrc(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		isExe         bool
		shouldContain []string
	}{
		{
			name:        "Executable",
			projectName: "myapp",
			isExe:       true,
			shouldContain: []string{
				"executable('myapp'",
				"static_library('myapp_lib'",
				"main.cpp",
				"myapp.cpp",
			},
		},
		{
			name:        "Library",
			projectName: "mylib",
			isExe:       false,
			shouldContain: []string{
				"library('mylib'",
				"mylib.cpp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateMesonBuildSrc(tt.projectName, tt.isExe)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateMesonBuildTests(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		testFramework string
		shouldContain []string
	}{
		{
			name:          "GoogleTest",
			projectName:   "myproject",
			testFramework: "googletest",
			shouldContain: []string{
				"dependency('gtest'",
				"executable('myproject_test'",
				"test('myproject tests'",
			},
		},
		{
			name:          "Catch2",
			projectName:   "myproject",
			testFramework: "catch2",
			shouldContain: []string{
				"dependency('catch2-with-main'",
				"executable('myproject_test'",
			},
		},
		{
			name:          "Doctest",
			projectName:   "myproject",
			testFramework: "doctest",
			shouldContain: []string{
				"dependency('doctest'",
				"executable('myproject_test'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateMesonBuildTests(tt.projectName, tt.testFramework)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateMesonBuildBench(t *testing.T) {
	tests := []struct {
		name               string
		projectName        string
		benchmarkFramework string
		shouldContain      []string
	}{
		{
			name:               "Google Benchmark",
			projectName:        "myproject",
			benchmarkFramework: "google-benchmark",
			shouldContain: []string{
				"dependency('benchmark'",
				"executable('myproject_bench'",
			},
		},
		{
			name:               "Nanobench",
			projectName:        "myproject",
			benchmarkFramework: "nanobench",
			shouldContain: []string{
				"# nanobench is header-only",
				"executable('myproject_bench'",
			},
		},
		{
			name:               "Catch2 Benchmark",
			projectName:        "myproject",
			benchmarkFramework: "catch2-benchmark",
			shouldContain: []string{
				"dependency('catch2-with-main'",
				"executable('myproject_bench'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateMesonBuildBench(tt.projectName, tt.benchmarkFramework)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateMesonOptions(t *testing.T) {
	result := GenerateMesonOptions()

	assert.Contains(t, result, "enable_tests")
	assert.Contains(t, result, "enable_benchmarks")
	assert.Contains(t, result, "type : 'boolean'")
}

func TestGenerateMesonGitignore(t *testing.T) {
	result := GenerateMesonGitignore()

	assert.Contains(t, result, "builddir/")
	assert.Contains(t, result, "build/")
	assert.Contains(t, result, ".cache/")
	assert.Contains(t, result, ".idea/")
}

func TestGenerateMesonReadme(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		cppStandard   int
		isLib         bool
		shouldContain []string
	}{
		{
			name:        "Executable",
			projectName: "myapp",
			cppStandard: 17,
			isLib:       false,
			shouldContain: []string{
				"# myapp",
				"C++17",
				"cpx build",
				"cpx run",
				"cpx test",
			},
		},
		{
			name:        "Library",
			projectName: "mylib",
			cppStandard: 20,
			isLib:       true,
			shouldContain: []string{
				"# mylib",
				"C++20",
				"library",
				"meson compile",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateMesonReadme(tt.projectName, tt.cppStandard, tt.isLib)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

// ============================================================================
// CMAKE TEMPLATE TESTS
// ============================================================================

func TestGenerateVcpkgCMakeLists(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		cppStandard   int
		isExe         bool
		includeTests  bool
		shouldContain []string
	}{
		{
			name:         "Executable with tests",
			projectName:  "myapp",
			cppStandard:  17,
			isExe:        true,
			includeTests: true,
			shouldContain: []string{
				"project(myapp",
				"CMAKE_CXX_STANDARD 17",
				"add_executable",
				"add_subdirectory(tests)",
			},
		},
		{
			name:         "Library without tests",
			projectName:  "mylib",
			cppStandard:  20,
			isExe:        false,
			includeTests: false,
			shouldContain: []string{
				"project(mylib",
				"CMAKE_CXX_STANDARD 20",
				"add_library",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateVcpkgCMakeLists(tt.projectName, tt.cppStandard, tt.isExe, tt.includeTests, "", false, "0.1.0")

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateCMakePresets(t *testing.T) {
	result := GenerateCMakePresets()

	assert.Contains(t, result, "configurePresets")
	assert.Contains(t, result, "VCPKG_ROOT")
	assert.Contains(t, result, "vcpkg.cmake")
}

func TestGenerateTestMain(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		testFramework string
		shouldContain []string
	}{
		{
			name:          "GoogleTest",
			projectName:   "myproject",
			testFramework: "googletest",
			shouldContain: []string{"gtest", "TEST("},
		},
		{
			name:          "Catch2",
			projectName:   "myproject",
			testFramework: "catch2",
			shouldContain: []string{"catch2", "TEST_CASE"},
		},
		{
			name:          "Doctest",
			projectName:   "myproject",
			testFramework: "doctest",
			shouldContain: []string{"doctest", "TEST_CASE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTestMain(tt.projectName, tt.testFramework)

			for _, s := range tt.shouldContain {
				assert.Contains(t, result, s, "Expected to contain: %s", s)
			}
		})
	}
}

func TestGenerateMainCpp(t *testing.T) {
	result := GenerateMainCpp("myproject")

	assert.Contains(t, result, "#include")
	assert.Contains(t, result, "myproject")
	assert.Contains(t, result, "int main")
}

func TestGenerateLibHeader(t *testing.T) {
	result := GenerateLibHeader("myproject")

	assert.Contains(t, result, "#ifndef")
	assert.Contains(t, result, "#define")
	assert.Contains(t, result, "namespace")
	assert.Contains(t, result, "myproject")
}

func TestGenerateLibSource(t *testing.T) {
	result := GenerateLibSource("myproject")

	assert.Contains(t, result, "#include")
	assert.Contains(t, result, "myproject")
	assert.Contains(t, result, "namespace")
}

func TestGenerateClangFormat(t *testing.T) {
	tests := []struct {
		style         string
		shouldContain string
	}{
		{"Google", "BasedOnStyle: Google"},
		{"LLVM", "BasedOnStyle: LLVM"},
		{"Chromium", "BasedOnStyle: Chromium"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			result := GenerateClangFormat(tt.style)
			assert.Contains(t, result, tt.shouldContain)
		})
	}
}

func TestGenerateCpxCI(t *testing.T) {
	result := GenerateCpxCI()

	assert.Contains(t, result, "toolchains:")
	assert.Contains(t, result, "build:")
}
