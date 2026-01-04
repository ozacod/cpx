<div align="center">

# <img src="cpx.svg" alt="cpx logo" width="30" /> cpx

[![GitHub release](https://img.shields.io/github/release/ozacod/cpx.svg)](https://github.com/ozacod/cpx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docs](https://img.shields.io/badge/docs-site-blue)](https://cpx-dev.vercel.app/docs)

**Cpx Your Code!** Cargo-like DX for C++: scaffold, build, test, bench, lint, package, and cross-compile with one CLI.
Supports **CMake (vcpkg)**, **Bazel**, and **Meson**.

Read the full docs at [cpx-dev.vercel.app/docs](https://cpx-dev.vercel.app/docs).

</div>

<p align="center">
  <img src="demo.gif" alt="cpx TUI demo" width="720" />
</p>



## Overview

`cpx` is a batteries-included CLI for C++ that unifies the fragmented C++ ecosystem. It provides a cohesive, Cargo-like experience for managing projects, dependencies, and builds, regardless of your underlying build system.

### Highlights
- **Interactive Scaffolding**: `cpx new` TUI to create projects with your preferred stack:
  - **Build Systems**: CMake (default), Bazel, Meson
  - **Test Frameworks**: GoogleTest, Catch2, Doctest
  - **Benchmarking**: Google Benchmark, Nanobench, Catch2
- **Dependency Management**:
  - `cpx add <pkg>` installs packages seamlessly:
    - **vcpkg** for CMake projects
    - **WrapDB** for Meson projects
    - **Bazel Central Registry** for Bazel projects 
- **Unified Workflow**: `cpx build`, `cpx run`, `cpx test`, `cpx bench` work consistently across all project types.
- **Code Quality**: Built-in support for `clang-format`, `clang-tidy`, `cppcheck`, and `flawfinder`.
- **Sanitizers**: Easy flags for ASan, TSan, MSan, UBSan.
- **Cross-Compilation**: Generate Docker-based toolchains with `cpx add-toolchain`.
- **Smart Tool Detection**: Automatically validates environment and warns about missing build tools.

## Install

### One-liner (recommended)
```bash
curl -fsSL https://raw.githubusercontent.com/ozacod/cpx/master/install.sh | sh
```
The installer downloads the latest binary, sets up vcpkg (if needed), and configures your environment.

### Manual
1. Download functionality for your OS from [Releases](https://github.com/ozacod/cpx/releases/latest).
2. Install it to your PATH:
```bash
chmod +x cpx-<os>-<arch>
mv cpx-<os>-<arch> /usr/local/bin/cpx
```
3. (Optional) Configure vcpkg root if you have an existing installation:
```bash
cpx config set-vcpkg-root /path/to/vcpkg
```

## Quick Start

### Create a New Project
Use the interactive TUI to generate a modern project structure:
```bash
cpx new
```

The wizard guides you through:
- **Project name**: Your project's name
- **Project mode**: Use Template or Custom Project
- **Project type**: Executable or Library
- **Build system**: vcpkg (default), Bazel, Meson, or CMake (standalone)
- **C++ standard**: 11, 14, 17, 20, 23
- **Test framework**: GoogleTest, Catch2, doctest, or None
- **Benchmark framework**: Google Benchmark, nanobench, Catch2 benchmark, or None
- **Code formatting**: Clang-format style (Google, LLVM, Chromium, Mozilla, WebKit)
- **Git repository**: Initialize git (Yes/No)
- **Pre-commit hooks**: format, lint, cppcheck, test (multi-select)
- **Pre-push hooks**: test, cppcheck (multi-select)

#### Available Templates
When using **Template** mode, choose from pre-configured project starters:

| Template    | Description                                    |
|-------------|------------------------------------------------|
| SDL2        | Game/multimedia application with SDL2          |
| ImGui       | Immediate mode GUI application                 |
| Qt          | Cross-platform Qt application                  |
| Raylib      | Simple game development with Raylib            |
| SFML        | Multimedia/game application with SFML          |
| Vulkan      | Graphics application with Vulkan API           |
| OpenCV      | Computer vision application                    |
| gRPC        | High-performance RPC services                  |
| REST API    | RESTful web service                            |
| CLI         | Command-line application with argument parsing |
| Audio       | Audio processing application                   |
| Game Engine | Basic game engine starter                      |
| WebAssembly | Browser-based C++ with Emscripten              |

### Common Commands
All commands auto-detect the project type (`vcpkg.json`, `MODULE.bazel`, or `meson.build`).

```bash
# Build & Run
cpx build            # Debug build
cpx build --release  # Release build (-O2/optimized)
cpx run              # specific generated executable

# Test & Bench
cpx test             # Run unit tests
cpx bench            # Run benchmarks

# Dependencies
cpx add fmt          # Install a package (vcpkg/WrapDB/Bazel)
cpx remove fmt       # Remove a package

# Quality
cpx fmt              # Format code
cpx lint             # Run linter
```

## Supported Build Systems

### CMake + vcpkg (Default)
The gold standard for modern C++. `cpx` manages `vcpkg.json` and cmake configuration for you.
- **Add deps**: `cpx add nlohmann-json` updates `vcpkg.json`.- **Build**: Uses CMake with vcpkg toolchain file.

#### Standalone CMake
`cpx` also supports standard CMake projects without vcpkg. If `vcpkg.json` is missing but `CMakeLists.txt` is present, `cpx` will treat it as a standalone CMake project.

### Meson
Fast and user-friendly. `cpx` wraps `meson setup`, `compile`, and dependency management via WrapDB.
- **Add deps**: `cpx add spdlog` runs `meson wrap install spdlog`.
- **Build**: Manages `builddir` configuration automatically.

### Bazel
Google's multi-language build system. `cpx` manages `MODULE.bazel` (Bzlmod).
- **Add deps**: `cpx add abseil-cpp` adds to `bazel_dep`.
- **Build**: Wraps `bazel build` and normalizes artifact output.

## Command Reference

| Command                    | Description                                                            |
|----------------------------|------------------------------------------------------------------------|
| `new`                      | Interactive project creation wizard                                    |
| `add <pkg>`                | Add a dependency (supports vcpkg, WrapDB, Bazel)                       |
| `remove <pkg>`             | Remove a dependency                                                    |
| `build`                    | Compile project (`--release`, `--asan`, `--tsan`, `--msan`, `--ubsan`) |
| `build --toolchain <name>` | Build using a toolchain in Docker (from cpx-ci.yaml)                   |
| `build all`                | Build all toolchains using Docker (from cpx-ci.yaml)                   |
| `run`                      | Build and run executable (`--asan`, `--tsan`, `--msan`, `--ubsan`)     |
| `run --toolchain <name>`   | Build and run in Docker toolchain                                      |
| `test`                     | Run tests (`--filter`)                                                 |
| `test --toolchain <name>`  | Run tests in Docker toolchain                                          |
| `bench`                    | Run benchmarks                                                         |
| `bench --toolchain <name>` | Run benchmarks in Docker toolchain                                     |
| `fmt`                      | Format code using `clang-format`                                       |
| `lint`                     | Lint code using `clang-tidy`                                           |
| `clean`                    | Remove build artifacts                                                 |
| `search`                   | Search for libraries interactively                                     |
| `info <pkg>`               | Show detailed library information                                      |
| `list`                     | List available libraries                                               |
| `update`                   | Check for outdated dependencies                                        |
| `upgrade`                  | Upgrade dependencies to newer versions                                 |
| `doc`                      | Generate documentation                                                 |
| `release`                  | Bump version number                                                    |
| `hooks`                    | Install git hooks                                                      |
| `workflow`                 | Generate CI/CD workflow files                                          |
| `self-upgrade`             | Self-update cpx to the latest version                                  |
| `env`                      | Print environment information (OS, compilers, tools)                   |

### Cross-Compilation & Toolchains

Manage Docker-based build toolchains defined in `cpx-ci.yaml`. `cpx` provides a clean build output by default when using toolchains, only showing the final result.

| Command                    | Description                                      |
|----------------------------|--------------------------------------------------|
| `add-toolchain`            | Interactive wizard to add build configurations   |
| `add-runner`               | Interactive wizard to add execution environments |
| `rm-toolchain [name...]`   | Remove toolchain(s) from cpx-ci.yaml             |
| `rm-runner [name...]`      | Remove runner(s) from cpx-ci.yaml                |
| `build --toolchain <name>` | Build using Docker (`--verbose` for full output) |
| `run --toolchain <name>`   | Build and run in Docker (quiet build by default) |
| `test --toolchain <name>`  | Run tests in Docker                              |
| `bench --toolchain <name>` | Run benchmarks in Docker                         |

#### `cpx-ci.yaml` Configuration

A toolchain configuration consists of **Runners** (where it runs) and **Toolchains** (how it builds).

```yaml
# execution environments
runners:
  - name: ubuntu-22.04
    type: docker           # docker, native, ssh
    image: cpx-linux:latest

# build configurations
toolchains:
  - name: linux-release
    runner: ubuntu-22.04
    optimization: "3"       # 0, 1, 2, 3, s, fast (default: 2)
    jobs: 8                 # Number of parallel jobs (default: auto)
    build_type: "Release"   # Debug, Release, RelWithDebInfo
    cc: gcc-13              # Compiler overrides
    cxx: g++-13
    cmake_toolchain_file: /opt/toolchain.cmake
```

**Runners** decouple the build environment from the build configuration, allowing you to reuse the same Docker image or SSH target for multiple toolchains (e.g., Debug vs Release builds on the same runner).

### Config Commands (`cpx config`)

| Command                 | Description              |
|-------------------------|--------------------------|
| `config set-vcpkg-root` | Set vcpkg root directory |
| `config show`           | Show all config values   |

### Self-Upgrade Commands (`cpx self-upgrade`)

| Command              | Description                           |
|----------------------|---------------------------------------|
| `self-upgrade`       | Self-update cpx to the latest version |
| `self-upgrade vcpkg` | Update vcpkg via git pull + bootstrap |

### Dependency Commands

| Command   | Description                                      |
|-----------|--------------------------------------------------|
| `update`  | Check for outdated packages (like `vcpkg update`)|
| `upgrade` | Upgrade packages to newer versions               |

## Shortcuts Cheat Sheet

| Command                         | Flag          | Shorthand   |
|:--------------------------------|:--------------|:------------|
| `build`                         |               | `b`         |
| `run`                           |               | `r`         |
| `test`                          |               | `t`         |
| `bench`                         |               | `be`        |
| `add`                           |               | `a`         |
| `remove`                        |               | `rm`        |
| `clean`                         |               | `cl`, `cls` |
| `build`, `run`, `test`, `bench` | `--toolchain` | `-t`        |
| `build`, `run`                  | `--verbose`   | `-v`        |
| `run`                           | `--release`   | `-r`        |
| `test`                          | `--filter`    | `-f`        |
| `fmt`                           | `--check`     | `-c`        |
| `lint`                          | `--fix`       | `-f`        |

## Roadmap
- SSH support for remote builds
- Multi target configuration
- CMake hierarchy detection with the help of tree-sitter
- ccache integration

## Contributing
Issues and PRs are welcome!
- **Docs**: [cpx-dev.vercel.app/docs](https://cpx-dev.vercel.app/docs)
- **Repo**: [github.com/ozacod/cpx](https://github.com/ozacod/cpx)

## License
MIT. See [LICENSE](LICENSE).
