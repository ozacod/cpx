---
sidebar_position: 2
---

# Best Practices

Recommendations for getting the most out of cpx and C++ development.

## Project Structure

### Recommended Layout

```
my_project/
├── CMakeLists.txt          # Main CMake configuration
├── CMakePresets.json       # CMake presets (auto-generated)
├── vcpkg.json              # Dependencies
├── cpx.yaml                # Project configuration
├── .clang-format           # Code style (optional)
├── .clang-tidy             # Linting rules (optional)
├── include/                # Public headers
│   └── my_project/
│       └── my_project.hpp
├── src/                    # Source files
│   ├── main.cpp
│   └── my_project.cpp
├── tests/                  # Test files
│   └── test_main.cpp
└── docs/                   # Documentation (optional)
```

### Header Organization

- Keep public headers in `include/<project_name>/`
- Use `#pragma once` or include guards
- Prefer forward declarations when possible

```cpp
// include/my_project/my_project.hpp
#pragma once

namespace my_project {

class MyClass {
public:
    void doSomething();
};

} // namespace my_project
```

## Dependency Management

### Prefer vcpkg Packages

Use vcpkg packages instead of vendoring dependencies:

```bash
# Good: Use vcpkg
cpx add port spdlog
cpx add port fmt

# Avoid: Don't vendor dependencies manually
```

### Pin Versions for Production

For production projects, consider specifying versions in `vcpkg.json`:

```json
{
  "dependencies": [
    {
      "name": "spdlog",
      "version>=": "1.12.0"
    }
  ]
}
```

### Use Feature Flags

Enable only what you need:

```json
{
  "dependencies": [
    {
      "name": "boost",
      "features": ["filesystem", "system"]
    }
  ]
}
```

## Build Configuration

### Use Optimization Levels Appropriately

| Level | Use Case |
|-------|----------|
| `-O0` | Debugging (default) |
| `-O1` | Light optimization, faster compile |
| `-O2` | Production (release default) |
| `-O3` | Maximum performance |
| `-Os` | Size optimization |

```bash
# Development
cpx build

# Release
cpx build --release

# Maximum performance
cpx build -O3
```

### Use Parallel Builds

Speed up builds with parallel compilation:

```bash
# Auto-detect CPU cores
cpx build

# Specify cores
cpx build -j 8
```

### Clean Builds for CI

Always use clean builds in CI to ensure reproducibility:

```bash
cpx build --clean
```

## Code Quality

### Use Git Hooks

Configure hooks in `cpx.yaml`:

```yaml
hooks:
  precommit:
    - fmt      # Format before commit
    - lint     # Lint before commit
  prepush:
    - test     # Test before push
```

### Create a .clang-format File

Define your code style:

```yaml
# .clang-format
BasedOnStyle: Google
IndentWidth: 4
ColumnLimit: 100
```

### Run Static Analysis Regularly

```bash
# Format code
cpx fmt

# Check for issues
cpx lint

# Security analysis
cpx flawfinder
cpx cppcheck
```

## Testing

### Write Tests Early

Use TDD or at minimum add tests for new features:

```cpp
// tests/test_main.cpp
#include <gtest/gtest.h>
#include <my_project/my_project.hpp>

TEST(MyClass, DoSomething) {
    my_project::MyClass obj;
    // Test implementation
}
```

### Use Verbose Mode for Debugging

```bash
cpx test -v
```

### Filter Tests During Development

```bash
cpx test --filter MyClass
```

## CI/CD

### GitHub Actions Workflow

Create `.github/workflows/build.yml`:

```yaml
name: Build
on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install cpx
        run: curl -f https://raw.githubusercontent.com/ozacod/cpx/master/install.sh | sh
      
      - name: Build
        run: cpx build --release
      
      - name: Test
        run: cpx test
```

### Cache vcpkg Packages

```yaml
- name: Cache vcpkg
  uses: actions/cache@v3
  with:
    path: ~/.cache/vcpkg
    key: ${{ runner.os }}-vcpkg-${{ hashFiles('vcpkg.json') }}
```

## Performance Tips

### Avoid Rebuilding vcpkg

Set environment variables to cache dependencies:

```bash
export VCPKG_BINARY_SOURCES="clear;default,readwrite"
export VCPKG_DISABLE_REGISTRY_UPDATE=1
```

### Use Precompiled Headers

For large projects, use precompiled headers in CMake:

```cmake
target_precompile_headers(my_project PRIVATE
    <string>
    <vector>
    <memory>
)
```

### Use Watch Mode for Development

```bash
cpx build --watch
```

## Security

### Run Security Checks

```bash
# Run Flawfinder
cpx flawfinder

# Run Cppcheck
cpx cppcheck --enable all
```

### Use Sanitizers in Development

```bash
# Memory errors
cpx check --asan

# Thread safety
cpx check --tsan

# Undefined behavior
cpx check --ubsan
```

### Keep Dependencies Updated

Regularly update your dependencies:

```bash
cpx upgrade
```
