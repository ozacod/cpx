# Build & Run Commands

Build, run, test, and benchmark with one CLI. All commands auto-detect your build system.

## cpx build
Compile your project. Uses the appropriate build system automatically.

```bash
cpx build                     # Debug build
cpx build --release           # Release build (-O2/optimized)
cpx build -O3                 # Maximum optimization
cpx build -j 8                # Parallel build (8 jobs)
cpx build --clean             # Clean then build
cpx build --target <name>     # Build specific target
```

![cpx build demo](/img/demo-build.gif)

### Sanitizer Builds
```bash
cpx build --asan    # AddressSanitizer (memory errors)
cpx build --tsan    # ThreadSanitizer (data races)
cpx build --msan    # MemorySanitizer (uninitialized memory)
cpx build --ubsan   # UndefinedBehaviorSanitizer
```

## cpx run
Build and run your executable. Arguments after `--` are passed to your program.

```bash
cpx run                       # Build and run (debug)
cpx run --release             # Build and run (release)
cpx run --asan                # Run with AddressSanitizer
cpx run --tsan                # Run with ThreadSanitizer
cpx run -- --flag value       # Pass args to your program
```

## cpx test
Build and run your test suite.

```bash
cpx test                      # Run all tests
cpx test -v                   # Verbose output
cpx test --filter <pattern>   # Filter tests by name
```

**Supported test frameworks:**
- GoogleTest
- Catch2
- Doctest

## cpx bench
Build and run benchmarks.

```bash
cpx bench                     # Run benchmarks
cpx bench --verbose           # Show verbose output
cpx bench --release           # Run in release mode
```

**Supported benchmark frameworks:**
- Google Benchmark
- Catch2 Benchmark
- nanobench

## cpx clean
Remove build artifacts.

```bash
cpx clean           # Remove build directory
cpx clean --all     # Also remove generated files
```
