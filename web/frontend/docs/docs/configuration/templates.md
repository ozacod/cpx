# Project Templates

Cpx still relies on templates internally, but you no longer pass template files or flags. Run the TUI with `cpx new`, choose your options, and the CLI applies the right template automatically.

## How it works

1. Run `cpx new`
2. Pick executable or library
3. Select a test framework (or none)
4. Choose git hook checks and formatting style

The CLI downloads the matching template and fills in your answers—no `cpx.yaml` required.

## Available Templates

### Default (GoogleTest)

Uses GoogleTest with standard git hook options and vcpkg manifest defaults. clang-format style is set from your TUI choice.

### Catch2

Uses Catch2 with the same build and hook scaffolding. Catch2 is pulled in via FetchContent automatically.

### No-test option

If you choose "no tests" in the TUI, the project is generated without testing targets while keeping the rest of the layout intact.

## Template Features

- Automatic download and application based on TUI choices
- Test framework selection in the TUI (GoogleTest, Catch2, or none)
- Git hook options captured in the TUI
- Build configuration driven by your answers (C++ standard, shared/static, clang-format style)
