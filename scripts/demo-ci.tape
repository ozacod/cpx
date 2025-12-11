# CPX Demo - CI Cross-Compilation
# Run with: vhs demo-ci.tape
# Requires: VHS (https://github.com/charmbracelet/vhs)

# All Set directives MUST be at the top (before any actions)
Output demo-ci.gif
Set FontSize 32
Set Width 1200
Set Height 800
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Setup: clean slate
Hide
Type "cd /tmp && rm -rf demo-project 2>/dev/null"
Enter
Sleep 300ms
Show

# === PART 1: Project Creation ===
Type "# Create a new C++ project in seconds"
Sleep 1s
Ctrl+C
Type "cpx new"
Enter
Sleep 500ms

# Project name
Type "demo-project"
Enter
Sleep 300ms

# Project type (Executable)
Enter
Sleep 200ms

# C++ standard (C++17)
Enter
Sleep 200ms

# Test framework (GoogleTest)
Enter
Sleep 200ms

# Benchmark framework (GoogleBenchmark)
Enter
Sleep 200ms

# clang-format style (Google)
Enter
Sleep 200ms

# Package manager (vcpkg)
Enter
Sleep 200ms

# Git + hooks (Yes)
Enter
Sleep 200ms

# Pre-commit hooks
Enter
Sleep 200ms

# Pre-push hooks
Enter
Sleep 1s

# === PART 2: Add Dependencies ===
Type "cd demo-project"
Enter
Sleep 300ms

Type "# Add dependencies with one command"
Sleep 1s
Ctrl+C

Type "cpx add spdlog"
Enter
Sleep 2s

Type "cat vcpkg.json"
Enter
Sleep 1s

# === PART 3: Build ===
Type "# Build the project"
Sleep 1s
Ctrl+C

Type "cpx build"
Enter
Sleep 3s

# === PART 4: Run ===
Type "# Run the executable"
Sleep 1s
Ctrl+C

Type "cpx run"
Enter
Sleep 1s

# === PART 5: Cross-compile (Docker) ===
Type "# Add CI targets for cross-compilation"
Sleep 1s
Ctrl+C

Type "cpx ci add-target linux-amd64 linux-arm64"
Enter
Sleep 1s

Type "# Build for ARM64"
Sleep 1s
Ctrl+C

Type "cpx ci --target linux-arm64"
Enter
Sleep 5s

# Final message
Type "# That's cpx - Simple C++ project management"
Sleep 2s
