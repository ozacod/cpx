package project_templates

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	build "github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipBuildTemplates contains templates that take too long to build their dependencies
// These are still tested for generation, but not for build
var skipBuildTemplates = map[string]bool{
	"gRPC":   true, // grpc + protobuf + abseil take very long
	"Qt":     true, // Qt5 is massive
	"OpenCV": true, // OpenCV takes a very long time
}

func TestProjectTemplates_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check for CMake
	_, err := exec.LookPath("cmake")
	if err != nil {
		t.Skip("cmake not found, skipping integration tests")
	}

	// Check for vcpkg
	vcpkgExec, err := exec.LookPath("vcpkg")
	hasVcpkg := err == nil

	if hasVcpkg && os.Getenv("VCPKG_ROOT") == "" {
		// Try to deduce VCPKG_ROOT from vcpkg executable path
		realPath, err := filepath.EvalSymlinks(vcpkgExec)
		if err == nil {
			os.Setenv("VCPKG_ROOT", filepath.Dir(realPath))
		}
	}

	if !hasVcpkg {
		t.Skip("vcpkg not found, skipping integration tests")
	}

	// We need to iterate over all registered templates
	// Note: Registry is populated in init() functions of other files in this package
	for _, info := range Registry {
		t.Run(info.Name, func(t *testing.T) {
			// Setup temp directory
			tmpDir, err := os.MkdirTemp("", "cpx-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			projectName := "test_proj"
			projectPath := filepath.Join(tmpDir, projectName)

			// Config
			config := TemplateConfig{
				ProjectName:    projectName,
				PackageManager: "vcpkg",
				CppStandard:    17,
			}

			// Change to temp dir so Generate writes there
			cwd, err := os.Getwd()
			require.NoError(t, err)
			defer func() {
				_ = os.Chdir(cwd)
			}()

			err = os.Chdir(tmpDir)
			require.NoError(t, err)

			// Generate the project
			t.Logf("Generating template %s...", info.Name)
			err = info.Template.Generate(config)
			require.NoError(t, err, "Failed to generate template")

			// Check basic file structure
			assert.FileExists(t, filepath.Join(projectPath, "CMakeLists.txt"))
			assert.FileExists(t, filepath.Join(projectPath, "README.md"))

			// Special handling for WebAssembly
			if strings.EqualFold(info.Name, "WebAssembly") {
				// WebAssembly template uses a custom build script with emcmake
				assert.FileExists(t, filepath.Join(projectPath, "build.sh"))
				t.Logf("Skipping build for %s (requires emcmake)", info.Name)
				return
			}

			// All other templates should have cpx-ci.yaml
			assert.FileExists(t, filepath.Join(projectPath, "cpx-ci.yaml"))

			// Skip build for templates with heavy dependencies
			if skipBuildTemplates[info.Name] {
				t.Logf("Skipping build for %s (heavy dependencies)", info.Name)
				return
			}

			// We need to change into the project directory for the builder to work
			err = os.Chdir(projectPath)
			require.NoError(t, err)

			// Use vcpkg builder to verify build/test/clean
			builder := vcpkg.New()

			// Build with 6 parallel jobs
			t.Log("Building...")
			err = builder.Build(build.BuildOptions{
				Release: false,
				Clean:   true,
				Jobs:    6,
				Verbose: testing.Verbose(),
			})
			require.NoError(t, err, "Build failed")

			// Clean
			t.Log("Cleaning...")
			err = builder.Clean(build.CleanOptions{All: true})
			assert.NoError(t, err, "Clean failed")
		})
	}
}
