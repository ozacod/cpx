# Build & Run Commands

Commands for building and running your C++ projects.

## cpx build

Compile the project using CMake presets if available.

```bash
cpx build
```

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--release` | `-r` | Build in release mode (O2) |
| `--debug` | | Build in debug mode (default) |
| `--opt <level>` | `-O` | Optimization: 0, 1, 2, 3, s, fast |
| `--clean` | `-c` | Clean and rebuild |
| `--jobs <n>` | `-j` | Parallel jobs (0 = auto) |
| `--target <name>` | | Build specific target |
| `--watch` | `-w` | Watch for changes and rebuild |

### Examples

```bash
# Debug build (default)
cpx build

# Release build
cpx build --release

# Maximum optimization
cpx build -O3

# Size optimization
cpx build -Os

# Parallel build with 8 cores
cpx build -j 8

# Clean rebuild
cpx build --clean

# Build specific target
cpx build --target my_lib

# Watch mode - rebuild on file changes
cpx build --watch

# Combine options
cpx build --release -O3 -j 8
```

### Optimization Levels

| Level | Flags | Use Case |
|-------|-------|----------|
| 0 | -O0 | Debugging, fast compile |
| 1 | -O1 | Light optimization |
| 2 | -O2 | Production (release default) |
| 3 | -O3 | Maximum performance |
| s | -Os | Minimize binary size |
| fast | -Ofast | Aggressive optimization |

## cpx run

Build and run the executable.

```bash
cpx run
```

### Options

| Option | Description |
|--------|-------------|
| `--release` | Run in release mode |
| `--target <name>` | Run specific target |

### Passing Arguments

Arguments after `--` are passed to your executable:

```bash
cpx run -- arg1 arg2 --flag
```

### Examples

```bash
# Build and run
cpx run

# Run in release mode
cpx run --release

# Run specific target
cpx run --target my_app

# Pass arguments
cpx run -- --config debug --verbose

# Pass multiple arguments
cpx run -- input.txt output.txt
```

## cpx test

Build and run tests using CTest.

```bash
cpx test
```

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--verbose` | `-v` | Show verbose output |
| `--filter <name>` | | Filter tests by name |

### Examples

```bash
# Run all tests
cpx test

# Verbose output
cpx test -v

# Filter by test name
cpx test --filter MyTestCase

# Filter by pattern
cpx test --filter "Integration*"
```

## cpx check

Build with sanitizers to detect runtime issues.

```bash
cpx check
```

### Sanitizer Options

| Option | Description |
|--------|-------------|
| `--asan` | AddressSanitizer (memory errors) |
| `--tsan` | ThreadSanitizer (data races) |
| `--msan` | MemorySanitizer (uninitialized memory) |
| `--ubsan` | UndefinedBehaviorSanitizer |

### Examples

```bash
# Check for memory errors
cpx check --asan
./build/my_app

# Check for data races
cpx check --tsan
./build/my_app

# Check for undefined behavior
cpx check --ubsan
./build/my_app
```

:::note
Only one sanitizer can be active at a time. Sanitizers significantly slow down execution but catch bugs that are hard to find otherwise.
:::

## cpx clean

Remove build artifacts.

```bash
cpx clean
```

### Options

| Option | Description |
|--------|-------------|
| `--all` | Also remove generated files |

### Examples

```bash
# Remove build directory
cpx clean

# Remove build + generated files
cpx clean --all
```

