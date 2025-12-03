---
sidebar_position: 1
---

# Troubleshooting

Common issues and their solutions.

## Installation Issues

### "vcpkg_root not set"

**Error:**
```
vcpkg_root not set in config. Run: cpx config set-vcpkg-root <path>
```

**Solution:**
Configure the path to your vcpkg installation:
```bash
cpx config set-vcpkg-root /path/to/vcpkg
```

If you don't have vcpkg installed, the installer will clone it automatically:
```bash
curl -f https://raw.githubusercontent.com/ozacod/cpx/master/install.sh | sh
```

### "cpx: command not found"

**Solution:**
Add cpx to your PATH. The installer adds it to `/usr/local/bin` by default.

For manual installation:
```bash
# Move to a directory in your PATH
sudo mv cpx /usr/local/bin/

# Or add to .zshrc/.bashrc
export PATH="$PATH:/path/to/cpx"
```

## Build Issues

### "cmake configure failed"

**Possible causes:**

1. **Missing CMake:** Install CMake 3.15+
   ```bash
   # macOS
   brew install cmake
   
   # Ubuntu/Debian
   sudo apt install cmake
   ```

2. **Missing compiler:** Install a C++ compiler
   ```bash
   # macOS (Xcode Command Line Tools)
   xcode-select --install
   
   # Ubuntu/Debian
   sudo apt install build-essential
   ```

3. **vcpkg not configured:** Set vcpkg root
   ```bash
   cpx config set-vcpkg-root /path/to/vcpkg
   ```

### "vcpkg toolchain file not found"

**Error:**
```
vcpkg toolchain file not found: /path/to/vcpkg/scripts/buildsystems/vcpkg.cmake
```

**Solution:**
Your vcpkg installation may be incomplete. Bootstrap vcpkg:
```bash
cd /path/to/vcpkg
./bootstrap-vcpkg.sh  # or bootstrap-vcpkg.bat on Windows
```

### "dependency not found"

**Error:**
```
Could not find package configuration file provided by "spdlog"
```

**Solution:**
Dependencies are installed during the first build. If issues persist:

1. Clean and rebuild:
   ```bash
   cpx clean --all
   cpx build
   ```

2. Manually install the dependency:
   ```bash
   cpx add port spdlog
   cpx build
   ```

### Build takes too long

**Tips:**

1. Use parallel builds:
   ```bash
   cpx build -j 8  # Use 8 cores
   ```

2. Skip vcpkg registry updates:
   ```bash
   export VCPKG_DISABLE_REGISTRY_UPDATE=1
   cpx build
   ```

3. Use binary caching (vcpkg feature):
   ```bash
   export VCPKG_BINARY_SOURCES="clear;default,readwrite"
   ```

## Code Quality Issues

### "clang-tidy not found"

**Solution:**
Install LLVM/Clang:
```bash
# macOS
brew install llvm

# Ubuntu/Debian
sudo apt install clang-tidy

# Add to PATH if needed
export PATH="/usr/local/opt/llvm/bin:$PATH"
```

### "clang-format not found"

**Solution:**
```bash
# macOS
brew install clang-format

# Ubuntu/Debian
sudo apt install clang-format
```

### "compile_commands.json not found"

**Error:**
```
compile_commands.json not found. Run 'cpx build' first
```

**Solution:**
Build the project first to generate the compilation database:
```bash
cpx build
cpx lint  # Now should work
```

## Test Issues

### "tests not found"

**Possible causes:**

1. **No test target:** Ensure your CMakeLists.txt defines a test target:
   ```cmake
   add_executable(${PROJECT_NAME}_tests tests/test_main.cpp)
   ```

2. **Wrong target name:** cpx looks for `<project_name>_tests`

3. **Tests not built:** Build tests explicitly:
   ```bash
   cpx build --target myproject_tests
   ```

### "ctest not found"

**Solution:**
CTest comes with CMake. Make sure CMake is properly installed.

## Git Hook Issues

### "not in a git repository"

**Solution:**
Initialize a git repository:
```bash
git init
cpx hooks install
```

### "hooks not running"

**Possible causes:**

1. **Not executable:** Make hooks executable
   ```bash
   chmod +x .git/hooks/pre-commit
   chmod +x .git/hooks/pre-push
   ```

2. **cpx not in PATH:** Hooks call cpx directly. Ensure cpx is in your PATH.

3. **Wrong shell:** Hooks use bash. Make sure bash is available.

## Getting More Help

### Enable debug output

```bash
export CPX_DEBUG=1
cpx build
```

### Check version

```bash
cpx version
```

### Report issues

If you can't find a solution, [open an issue on GitHub](https://github.com/ozacod/cpx/issues) with:
- cpx version
- Operating system
- Error message
- Steps to reproduce
