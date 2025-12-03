---
sidebar_position: 3
---

# FAQ

Frequently asked questions about cpx.

## General

### What is cpx?

cpx is a modern C++ project generator and build tool that brings the developer experience of Rust's Cargo to C++. It simplifies project creation, dependency management, building, testing, and code quality.

### How is cpx different from CMake?

cpx uses CMake under the hood but provides a simpler, more opinionated interface. Think of cpx as a wrapper that:

- Generates CMake configurations automatically
- Integrates vcpkg for dependency management
- Provides consistent commands across projects
- Adds code quality tools built-in

### Is cpx production-ready?

Yes! cpx is designed for real-world C++ projects. It generates standard CMake projects that can be used with any CMake-compatible IDE or build system.

### What platforms does cpx support?

- **macOS**: Intel and Apple Silicon (arm64)
- **Linux**: x86_64 and arm64
- **Windows**: x86_64

## Installation

### How do I install cpx?

The recommended way is the install script:

```bash
curl -f https://raw.githubusercontent.com/ozacod/cpx/master/install.sh | sh
```

### How do I upgrade cpx?

```bash
cpx upgrade
```

### Where does cpx store configuration?

- **macOS/Linux**: `~/.config/cpx/config.yaml`
- **Windows**: `%APPDATA%\cpx\config.yaml`

## Dependencies

### How does cpx manage dependencies?

cpx uses [vcpkg](https://vcpkg.io/) for dependency management. Dependencies are specified in `vcpkg.json` and automatically installed during the first build.

### How do I add a dependency?

```bash
cpx add port spdlog
```

This runs `vcpkg add port spdlog` to add the package to your `vcpkg.json`.

### Can I use dependencies from other sources?

Yes! vcpkg supports:
- Official vcpkg registry (default)
- Custom registries
- Git repositories
- Local packages

### Are dependencies cached?

Yes. vcpkg caches built packages. Enable binary caching for faster CI builds:

```bash
export VCPKG_BINARY_SOURCES="clear;default,readwrite"
```

## Building

### How do I build in release mode?

```bash
cpx build --release
```

### How do I control optimization?

```bash
cpx build -O3  # Maximum optimization
cpx build -Os  # Size optimization
```

### How do I clean the build?

```bash
cpx clean         # Remove build directory
cpx clean --all   # Also remove generated files
```

### Can I use Ninja instead of Make?

Yes! CMake uses the default generator. You can specify Ninja in `CMakePresets.json`:

```json
{
  "configurePresets": [{
    "name": "default",
    "generator": "Ninja"
  }]
}
```

## Testing

### What testing frameworks are supported?

- **GoogleTest** (default)
- **Catch2**
- **doctest**

### How do I filter tests?

```bash
cpx test --filter MyTestCase
```

### How do I see test output?

```bash
cpx test -v  # Verbose mode
```

## Code Quality

### What tools are included?

- **clang-format**: Code formatting
- **clang-tidy**: Static analysis
- **Flawfinder**: Security analysis
- **Cppcheck**: Static analysis

### How do I configure clang-format?

Create a `.clang-format` file in your project root:

```yaml
BasedOnStyle: Google
IndentWidth: 4
```

### Are there git hooks?

Yes! Configure in `cpx.yaml`:

```yaml
hooks:
  precommit:
    - fmt
    - lint
  prepush:
    - test
```

Install with:
```bash
cpx hooks install
```

## Troubleshooting

### cpx says "vcpkg_root not set"

Configure vcpkg location:
```bash
cpx config set-vcpkg-root /path/to/vcpkg
```

### Build fails with "package not found"

Run a clean build:
```bash
cpx clean --all
cpx build
```

### clang-tidy says "compile_commands.json not found"

Build the project first:
```bash
cpx build
cpx lint
```

### Tests don't run

Ensure your project has a test target named `<project_name>_tests`.

## Contributing

### Where is the source code?

[github.com/ozacod/cpx](https://github.com/ozacod/cpx)

### How do I report bugs?

Open an issue on GitHub with:
- cpx version (`cpx version`)
- Operating system
- Error message
- Steps to reproduce

### How do I contribute?

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request
