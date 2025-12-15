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
    - **WrapDB** for Meson projects (via `meson wrap install`)
    - **Bazel Central Registry** for Bazel projects (via `MODULE.bazel`)
- **Unified Workflow**: `cpx build`, `cpx run`, `cpx test`, `cpx bench` work consistently across all project types.
- **Code Quality**: Built-in support for `clang-format`, `clang-tidy`, `cppcheck`, and `flawfinder`.
  - `cpx analyze` runs a comprehensive static analysis report.
- **Sanitizers**: Easy flags for ASan, TSan, MSan, UBSan.
- **CI/CD**: Generate Docker-based CI targets (Linux/Windows/Alpine) with `cpx ci`.

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
Select your build system (CMake, Bazel, Meson), project type (App/Lib), and test framework.

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
The gold standard for modern C++. `cpx` generates `CMakePresets.json` and manages `vcpkg.json` for you.
- **Add deps**: `cpx add nlohmann-json` updates `vcpkg.json`.
- **Build**: Uses CMake Presets (`debug`, `release`).

### Meson
Fast and user-friendly. `cpx` wraps `meson setup`, `compile`, and dependency management via WrapDB.
- **Add deps**: `cpx add spdlog` runs `meson wrap install spdlog`.
- **Build**: Manages `builddir` configuration automatically.

### Bazel
Google's multi-language build system. `cpx` manages `MODULE.bazel` (Bzlmod).
- **Add deps**: `cpx add abseil-cpp` adds to `bazel_dep`.
- **Build**: Wraps `bazel build` and normalizes artifact output.

## Command Reference

| Command | Description |
|---------|-------------|
| `new` | Interactive project creation wizard |
| `add <pkg>` | Add a dependency (supports vcpkg, WrapDB, Bazel) |
| `remove <pkg>` | Remove a dependency |
| `build` | Compile project (`--release`, `--asan`, `--tsan`, `--msan`, `--ubsan`) |
| `build all` | Build all CI targets using Docker (from cpx-ci.yaml) |
| `run` | Build and run executable (`--asan`, `--tsan`, `--msan`, `--ubsan`) |
| `test` | Run tests (`--filter`) |
| `bench` | Run benchmarks |
| `fmt` | Format code using `clang-format` |
| `lint` | Lint code using `clang-tidy` |
| `analyze` | Run static analysis (cppcheck, flawfinder) & report |
| `clean` | Remove build artifacts |
| `search` | Search for libraries interactively |
| `info <pkg>` | Show detailed library information |
| `list` | List available libraries |
| `update` | Update dependencies to latest versions |
| `doc` | Generate documentation |
| `release` | Bump version number |
| `hooks` | Install git hooks |
| `workflow` | Generate CI/CD workflow files |
| `upgrade` | Self-update to the latest version |

### Target Management

Manage cross-compilation targets (Docker-based builds). Requires `cpx-ci.yaml` configuration file.

| Command | Description |
|---------|-------------|
| `add-target` | Interactive wizard to add a new build target |
| `rm-target [targets...]` | Remove build target(s) from cpx-ci.yaml |
| `rm-target list` | Interactive target removal from cpx-ci.yaml |

### Config Commands (`cpx config`)

| Command | Description |
|---------|-------------|
| `config set-vcpkg-root` | Set vcpkg root directory |

### Upgrade Commands (`cpx upgrade`)

| Command | Description |
|---------|-------------|
| `upgrade` | Self-update cpx to the latest version |
| `upgrade vcpkg` | Update vcpkg via git pull + bootstrap |

## Contributing
Issues and PRs are welcome!
- **Docs**: [cpx-dev.vercel.app/docs](https://cpx-dev.vercel.app/docs)
- **Repo**: [github.com/ozacod/cpx](https://github.com/ozacod/cpx)

## License
MIT. See [LICENSE](LICENSE).
