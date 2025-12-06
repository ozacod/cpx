# Project Configuration

Project configuration is captured through the interactive TUI (`cpx new`). You no longer edit or supply a `cpx.yaml` file—the CLI generates everything it needs based on your answers.

## vcpkg.json

The `vcpkg.json` file is the vcpkg manifest that lists all dependencies. This file is auto-generated and managed via `cpx add port` commands.

```json
{
  "dependencies": [
    "spdlog",
    "fmt",
    "nlohmann-json"
  ]
}
```

## What the TUI captures

When you run `cpx new`, the TUI asks for:
- Project name and type (executable or library)
- Test framework
- Git hook checks
- C++ standard and formatting preference
- Package manager and VCS defaults

Those answers drive the generated files:
- `CMakeLists.txt` and `CMakePresets.json`
- `vcpkg.json`
- `.clang-format` (optional)
- `.gitignore`
- `cpx.ci` (empty targets by default)

