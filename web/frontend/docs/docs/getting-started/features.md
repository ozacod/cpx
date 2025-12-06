# Features

Cpx comes with a comprehensive set of features to streamline your C++ development workflow.

## 📦 vcpkg Integration

Direct integration with Microsoft vcpkg for dependency management. All vcpkg commands work seamlessly through Cpx.

## 🔧 CMake Presets

Automatic CMakePresets.json generation for seamless IDE integration with VS Code, CLion, and Qt Creator.

## ✨ Code Quality Tools

Built-in support for:
- **clang-format** - Code formatting
- **clang-tidy** - Static analysis
- **Flawfinder** - Security analysis
- **Cppcheck** - Static analysis

## 🔒 Security Analysis

Comprehensive security tools:
- Flawfinder for C/C++ security vulnerabilities
- Cppcheck for static analysis
- Sanitizer support (ASan, TSan, MSan, UBSan)

## 🧪 Testing Support

Automatic test framework setup:
- **GoogleTest** - Google's C++ testing framework
- **Catch2** - Modern C++ test framework
- **doctest** - Fast single-header testing framework

## 🔄 CI/CD Integration

Generate CI/CD workflows automatically:
- GitHub Actions
- GitLab CI
- Cross-compilation support

## 🐳 Cross-Compilation

Docker-based cross-compilation for multiple platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

## ⚡ vcpkg Passthrough

All vcpkg commands work directly through Cpx:
- `cpx install <package>`
- `cpx list`
- `cpx search <query>`

## 📝 Configurable Git Hooks

Select git hooks during `cpx new` and install them with a single command:
- Pre-commit hooks
- Pre-push hooks
- Customizable check commands

