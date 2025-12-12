---
sidebar_position: 2
---

# Best Practices

Pragmatic tips to keep cpx projects healthy and repeatable.

## Structure
- Keep public headers in `include/<project>/`, use include guards or `#pragma once`.
- Keep build artifacts out of the repo (`build/` stays gitignored).
- Generate `version.hpp` and keep `CMakeLists.txt` as the single source of truth for the version (cpx release updates both).

## Dependencies
- Prefer vcpkg ports over vendoring; use `features` to keep builds lean.
- Pin versions for production in `vcpkg.json` when stability matters.
- Avoid mixing package managers inside one project.

## Builds
- Debug locally (`cpx build`), release for perf (`cpx build --release`), and use sanitizers during development (`--asan`, `--tsan`, `--msan`, `--ubsan`).
- Use parallelism (`-j <n>`) but keep CI deterministic (`cpx build --clean`).
- Keep `CMakePresets.json` under version control; let IDEs consume it.

## Quality and security
- Install hooks (`cpx hooks install`) to enforce fmt/lint/tests before pushes.
- Run `cpx fmt` + `cpx lint` regularly; add `cpx flawfinder` or `cpx cppcheck` for deeper checks.
- Keep `.clang-format` and any lint config in the repo; avoid per-developer overrides.

## Testing
- Start with the generated GoogleTest/Catch2 scaffolding; keep tests close to code.
- Use `cpx test --filter <pattern>` to iterate faster; add `-v` for verbose runs.

## CI and cross-compilation
- Define targets in `cpx.ci`; include musl targets if you need Alpine images.
- Rebuild images sparingly (`cpx ci --rebuild`) and pin toolchain versions in Dockerfiles for reproducibility.

## Releases
- Use `cpx release <major|minor|patch>` so `CMakeLists.txt` and `version.hpp` stay in sync.
- Ship binaries built with the release workflow (tag-injected version); prefer `cpx upgrade` for consumers.

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
cpx build --asan

# Thread safety
cpx build --tsan

# Undefined behavior
cpx build --ubsan
```

### Keep Dependencies Updated

Regularly update your dependencies:

```bash
cpx upgrade
```
