package meson

import (
	"os"
	"path/filepath"
	"testing"

	build "github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create a minimal meson project
	mesonBuild := `project('test_project', 'cpp',
  version : '0.1.0',
  default_options : ['cpp_std=c++17'])

executable('test_project', 'src/main.cpp')
`
	require.NoError(t, os.WriteFile("meson.build", []byte(mesonBuild), 0644))

	// Create src directory and main.cpp
	require.NoError(t, os.MkdirAll("src", 0755))
	mainCpp := `#include <iostream>
int main() {
    std::cout << "Hello from Meson!" << std::endl;
    return 0;
}
`
	require.NoError(t, os.WriteFile("src/main.cpp", []byte(mainCpp), 0644))

	builder := New()

	// Test Debug Build
	err = builder.Build(build.BuildOptions{
		Release: false,
		Jobs:    4,
	})
	assert.NoError(t, err)

	// Verify builddir was created
	_, err = os.Stat("builddir")
	assert.NoError(t, err, "builddir should exist after build")

	// Verify executable exists
	_, err = os.Stat(".bin/native/debug/test_project")
	assert.NoError(t, err, "executable should exist in .bin/native/debug/")

	// Test Release Build with Clean
	err = builder.Build(build.BuildOptions{
		Release: true,
		Clean:   true,
		Jobs:    4,
	})
	assert.NoError(t, err)

	// Verify release executable exists
	_, err = os.Stat(".bin/native/release/test_project")
	assert.NoError(t, err, "executable should exist in .bin/native/release/")
}

func TestRun_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create a minimal meson project
	mesonBuild := `project('test_project', 'cpp',
  version : '0.1.0',
  default_options : ['cpp_std=c++17'])

executable('test_project', 'src/main.cpp')
`
	require.NoError(t, os.WriteFile("meson.build", []byte(mesonBuild), 0644))

	// Create src directory and main.cpp
	require.NoError(t, os.MkdirAll("src", 0755))
	mainCpp := `#include <iostream>
int main() {
    std::cout << "Hello from Meson Run!" << std::endl;
    return 0;
}
`
	require.NoError(t, os.WriteFile("src/main.cpp", []byte(mainCpp), 0644))

	builder := New()

	// Run should build and execute
	err = builder.Run(build.RunOptions{
		Release: false,
		Target:  "test_project",
	})
	assert.NoError(t, err)
}

func TestTest_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create a meson project with tests
	mesonBuild := `project('test_project', 'cpp',
  version : '0.1.0',
  default_options : ['cpp_std=c++17'])

executable('test_project', 'src/main.cpp')

test_exe = executable('test_runner', 'tests/test_main.cpp')
test('basic_test', test_exe)
`
	require.NoError(t, os.WriteFile("meson.build", []byte(mesonBuild), 0644))

	// Create src directory and main.cpp
	require.NoError(t, os.MkdirAll("src", 0755))
	mainCpp := `#include <iostream>
int main() { return 0; }
`
	require.NoError(t, os.WriteFile("src/main.cpp", []byte(mainCpp), 0644))

	// Create tests directory and test file
	require.NoError(t, os.MkdirAll("tests", 0755))
	testCpp := `#include <iostream>
int main() {
    std::cout << "Test passed!" << std::endl;
    return 0;
}
`
	require.NoError(t, os.WriteFile("tests/test_main.cpp", []byte(testCpp), 0644))

	builder := New()

	// Build first
	err = builder.Build(build.BuildOptions{Release: false, Jobs: 4})
	require.NoError(t, err)

	// Run tests
	err = builder.Test(build.TestOptions{
		Verbose: true,
	})
	assert.NoError(t, err)
}

func TestClean_Integration(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create meson.build
	require.NoError(t, os.WriteFile("meson.build", []byte("project('test', 'cpp')"), 0644))

	// Create directories that should be cleaned
	require.NoError(t, os.MkdirAll("builddir", 0755))
	require.NoError(t, os.MkdirAll("build", 0755))
	require.NoError(t, os.MkdirAll("subprojects/packagecache", 0755))
	require.NoError(t, os.MkdirAll("build-release", 0755))

	builder := New()

	// Clean without all flag
	err = builder.Clean(build.CleanOptions{All: false})
	assert.NoError(t, err)

	// builddir and build should be removed
	_, err = os.Stat("builddir")
	assert.True(t, os.IsNotExist(err), "builddir should be removed")
	_, err = os.Stat("build")
	assert.True(t, os.IsNotExist(err), "build should be removed")

	// Recreate for all test
	require.NoError(t, os.MkdirAll("builddir", 0755))
	require.NoError(t, os.MkdirAll("subprojects/packagecache", 0755))
	require.NoError(t, os.MkdirAll("build-release", 0755))

	// Clean with all flag
	err = builder.Clean(build.CleanOptions{All: true})
	assert.NoError(t, err)

	// All directories should be removed
	_, err = os.Stat("builddir")
	assert.True(t, os.IsNotExist(err), "builddir should be removed")
	_, err = os.Stat("subprojects/packagecache")
	assert.True(t, os.IsNotExist(err), "subprojects/packagecache should be removed")
	_, err = os.Stat("build-release")
	assert.True(t, os.IsNotExist(err), "build-release should be removed")
}

func TestAddDependency_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// meson wrap requires a meson.build to exist
	require.NoError(t, os.WriteFile("meson.build", []byte("project('test', 'cpp')"), 0644))

	builder := New()
	err := builder.AddDependency("zlib", "")
	assert.NoError(t, err)

	// Verify wrap file was created
	_, err = os.Stat("subprojects/zlib.wrap")
	assert.NoError(t, err, "zlib.wrap should be created")
}

func TestRemoveDependency_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	subprojectsDir := "subprojects"
	_ = os.MkdirAll(subprojectsDir, 0755)
	wrapFile := filepath.Join(subprojectsDir, "zlib.wrap")
	_ = os.WriteFile(wrapFile, []byte("[wrap-file]\n"), 0644)
	extractedDir := filepath.Join(subprojectsDir, "zlib")
	_ = os.MkdirAll(extractedDir, 0755)

	builder := New()
	err := builder.RemoveDependency("zlib")
	assert.NoError(t, err)

	_, err = os.Stat(wrapFile)
	assert.True(t, os.IsNotExist(err), "wrap file should be removed")
	_, err = os.Stat(extractedDir)
	assert.True(t, os.IsNotExist(err), "extracted directory should be removed")
}

func TestListDependencies_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	subprojectsDir := "subprojects"
	_ = os.MkdirAll(subprojectsDir, 0755)
	_ = os.WriteFile(filepath.Join(subprojectsDir, "zlib.wrap"), []byte("[wrap-file]\n"), 0644)
	_ = os.WriteFile(filepath.Join(subprojectsDir, "glib.wrap"), []byte("[wrap-file]\n"), 0644)

	builder := New()
	deps, err := builder.ListDependencies()
	assert.NoError(t, err)
	assert.Len(t, deps, 2)

	names := []string{deps[0].Name, deps[1].Name}
	assert.Contains(t, names, "zlib")
	assert.Contains(t, names, "glib")
}

func TestName(t *testing.T) {
	builder := New()
	assert.Equal(t, "meson", builder.Name())
}

func TestListTargets_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Create a meson project
	mesonBuild := `project('test_project', 'cpp',
  version : '0.1.0',
  default_options : ['cpp_std=c++17'])

executable('my_app', 'src/main.cpp')
`
	require.NoError(t, os.WriteFile("meson.build", []byte(mesonBuild), 0644))
	require.NoError(t, os.MkdirAll("src", 0755))
	require.NoError(t, os.WriteFile("src/main.cpp", []byte("int main() { return 0; }"), 0644))

	builder := New()

	// Build first to setup the project (this creates builddir properly)
	err := builder.Build(build.BuildOptions{Release: false, Jobs: 4})
	require.NoError(t, err)

	targets, err := builder.ListTargets()
	assert.NoError(t, err)
	assert.NotEmpty(t, targets)

	// Should contain our executable
	found := false
	for _, target := range targets {
		if target == "my_app (executable)" {
			found = true
			break
		}
	}
	assert.True(t, found, "my_app should be in targets list")
}
