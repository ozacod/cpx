<div align="center">
  <img src="cpx.svg" alt="cpx Logo" width="200"/>
  
  # cpx
  
  **Cpx Your Code!** A modern C++ project generator and build tool that brings the developer experience of Rust's Cargo to C++.
</div>

[![GitHub release](https://img.shields.io/github/release/ozacod/cpx.svg)](https://github.com/ozacod/cpx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 What is cpx?

cpx is a comprehensive CLI tool for C++ development that simplifies project creation, dependency management, building, testing, and code quality. It integrates seamlessly with **vcpkg** for dependency management and provides a modern workflow similar to Rust's Cargo.

### Key Features

- 🎯 **Project Templates**: Create projects from templates (default, catch2) downloaded from GitHub
- 📦 **vcpkg Integration**: Direct integration with Microsoft vcpkg for dependency management
- 🏗️ **CMake Presets**: Automatic CMakePresets.json generation for seamless IDE integration
- 🧪 **Testing Frameworks**: Support for Google Test, Catch2 (auto-downloaded via FetchContent)
- 🔍 **Code Quality Tools**: Built-in clang-format, clang-tidy, Flawfinder, Cppcheck
- 🛡️ **Sanitizers**: AddressSanitizer, ThreadSanitizer, MemorySanitizer, UBSan support
- 🪝 **Git Hooks**: Automatic git hooks installation with configurable pre-commit/pre-push checks
- 🐳 **Cross-Compilation**: Docker-based CI builds for multiple platforms
- 📚 **Documentation**: Interactive web documentation at [cpxcpp.vercel.app](https://cpxcpp.vercel.app)

## 📥 Installation

### Quick Install (Recommended)

Install with a single command (auto-detects your OS and architecture):

```bash
curl -f https://raw.githubusercontent.com/ozacod/cpx/master/install.sh | sh
```

The installer will:
- Download the latest cpx binary for your platform
- Set up vcpkg (clones and bootstraps if needed)
- Configure cpx with vcpkg root directory
- Add cpx to your PATH

### Manual Installation

1. Download the binary for your platform from [GitHub Releases](https://github.com/ozacod/cpx/releases/latest)
2. Make it executable and move to PATH:
   ```bash
   chmod +x cpx-<platform>
   sudo mv cpx-<platform> /usr/local/bin/cpx
   ```
3. Configure vcpkg:
   ```bash
   cpx config set-vcpkg-root /path/to/vcpkg
   ```

## 🎯 Quick Start

```bash
# Launch the interactive creator (TUI)
cpx new

# After the TUI finishes
cd <project_name>

# Build the project
cpx build

# Run the executable
cpx run

# Run tests
cpx test

# Add dependencies
cpx add port spdlog
cpx add port fmt

# Format code
cpx fmt

# Run static analysis
cpx lint
```

## 📋 Commands

### Project Management

```bash
cpx new                              # Launch interactive TUI to create a project
```

### Build & Run

```bash
cpx build                   # Compile the project
cpx build --release         # Build in release mode
cpx build -O3               # Build with O3 optimization
cpx build --clean           # Clean and rebuild
cpx build -j 8              # Use 8 parallel jobs

cpx run                     # Build and run executable
cpx run --release           # Run in release mode
cpx run -- arg1 arg2        # Pass arguments to executable

cpx test                    # Build and run tests
cpx test -v                 # Verbose test output
cpx test --filter <name>    # Filter tests by name

cpx check                   # Check code compiles
cpx check --asan            # Build with AddressSanitizer
cpx check --tsan            # Build with ThreadSanitizer
cpx check --msan            # Build with MemorySanitizer
cpx check --ubsan           # Build with UndefinedBehaviorSanitizer

cpx clean                   # Remove build artifacts
cpx clean --all             # Also remove generated files
```

### Dependency Management

```bash
# cpx-specific commands
cpx add port <package>      # Add dependency to vcpkg.json
cpx remove <package>        # Remove dependency
cpx list                    # List installed packages
cpx search <query>          # Search packages
cpx info <package>          # Show package information
cpx update                  # Update dependencies

# Direct vcpkg passthrough (all vcpkg commands work)
cpx install <package>       # Install package
cpx upgrade                 # Upgrade all packages
cpx show <package>          # Show package details
```

### Code Quality

```bash
cpx fmt                     # Format code with clang-format
cpx fmt --check             # Check formatting without modifying files

cpx lint                    # Run clang-tidy static analysis
cpx lint --fix              # Auto-fix issues where possible

cpx flawfinder              # Run Flawfinder security analysis
cpx flawfinder --html       # HTML report
cpx flawfinder --csv        # CSV output
cpx flawfinder --dataflow   # Enable dataflow analysis

cpx cppcheck                # Run Cppcheck static analysis
cpx cppcheck --xml          # XML report
cpx cppcheck --enable <checks>  # Enable specific checks
```

### Configuration & Utilities

```bash
cpx config set-vcpkg-root <path>  # Set vcpkg root directory
cpx config get-vcpkg-root         # Get current vcpkg root

cpx hooks install            # Install git hooks (pre-commit, pre-push, etc.)

cpx release <type>           # Bump version (major, minor, patch)
cpx upgrade                  # Upgrade cpx to latest version
cpx version                  # Show version
cpx doc                      # Generate documentation
```

### CI/CD

```bash
cpx ci                       # Build for all targets in cpx.ci
cpx ci --target <target>    # Build specific target
cpx ci --rebuild            # Force rebuild Docker images

cpx ci init --github-actions # Generate GitHub Actions workflow
cpx ci init --gitlab        # Generate GitLab CI configuration
```

## 📄 Project Templates

cpx still uses templates behind the scenes, but you no longer pass template files or flags. Run `cpx new`, pick your options in the TUI, and the CLI downloads and applies the correct template automatically.

### Available Templates

- **default**: GoogleTest
- **catch**: Catch2
- **no tests**: if you decline a test framework in the TUI

### How it works

1. Run `cpx new`
2. Choose executable or library layout
3. Select a test framework (or none)
4. Choose git hook checks and formatting style

The template is fetched and filled in with your choices—no `cpx.yaml` required.


## ⚙️ Configuration

### Global Configuration

cpx stores its global configuration in:
- **Linux/macOS**: `~/.config/cpx/config.yaml`
- **Windows**: `%APPDATA%/cpx/config.yaml`

```yaml
vcpkg_root: "/path/to/vcpkg"
```

### Project Configuration

Project settings are captured through the interactive TUI (`cpx new`). There is no `cpx.yaml` to edit—your answers drive the generated files.

**vcpkg.json** (auto-generated):
```json
{
  "dependencies": [
    "spdlog",
    "fmt",
    "nlohmann-json"
  ]
}
```

## 🪝 Git Hooks

cpx can automatically install git hooks for code quality checks:

### Configuration

Pick the checks you want when you run `cpx new` (fmt, lint, test, flawfinder, cppcheck, check).

### Installation

Install the hooks into `.git/hooks`:

```bash
cpx hooks install
```

### Supported Hook Checks

- `fmt` - Format code with clang-format
- `lint` - Run clang-tidy static analysis
- `test` - Run tests (blocking for pre-push)
- `flawfinder` - Run Flawfinder security analysis
- `cppcheck` - Run Cppcheck static analysis
- `check` - Run code check

### Behavior

- If you selected checks in the TUI, those hooks are installed (e.g., `pre-commit`, `pre-push`)
- If you skipped hook selection, cpx installs defaults: fmt + lint on pre-commit, test on pre-push

## 🐳 Cross-Compilation

cpx supports cross-compilation using Docker. Configure targets in `cpx.ci`:

```yaml
targets:
  - name: linux-amd64
    dockerfile: dockerfiles/Dockerfile.linux-amd64
  - name: linux-arm64
    dockerfile: dockerfiles/Dockerfile.linux-arm64
  - name: windows-amd64
    dockerfile: dockerfiles/Dockerfile.windows-amd64
  - name: darwin-amd64
    dockerfile: dockerfiles/Dockerfile.macos-amd64
  - name: darwin-arm64
    dockerfile: dockerfiles/Dockerfile.macos-arm64
```

Build for all targets:
```bash
cpx ci
```

## 📁 Project Structure

A typical cpx project structure:

```
my_project/
├── CMakeLists.txt          # Main CMake configuration
├── CMakePresets.json        # CMake presets for IDE integration
├── vcpkg.json              # vcpkg dependencies
├── cpx.ci                 # Cross-compilation targets (optional)
├── include/                # Header files
│   └── my_project/
│       └── my_project.hpp
├── src/                    # Source files
│   ├── main.cpp
│   └── my_project.cpp
├── tests/                  # Test files
│   ├── CMakeLists.txt
│   └── test_main.cpp
└── build/                  # Build directory (gitignored)
```

## 🛠️ Building from Source

### Prerequisites

- Go 1.21+ (for CLI client)
- Node.js 18+ (for web UI, optional)
- vcpkg (will be cloned during installation)

### Build CLI

```bash
# Build for current platform
cd cpx
go build -o cpx .

# Build for all platforms
cd ..
make build-all
```

## 🌐 Web Documentation

Interactive documentation is available at [cpxcpp.vercel.app](https://cpxcpp.vercel.app), featuring:
- Comprehensive command reference
- Configuration guides
- Code quality tool documentation
- CI/CD setup instructions
- Examples and tutorials

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🔗 Links

- **Documentation**: [cpxcpp.vercel.app](https://cpxcpp.vercel.app)
- **Releases**: [github.com/ozacod/cpx/releases](https://github.com/ozacod/cpx/releases)

---

**Cpx Your Code!** 🔨
