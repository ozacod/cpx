# Code Quality & Security

Examples of using code quality and security tools.

## Configure Git Hooks

Choose the checks (fmt, lint, test, etc.) when you run `cpx new`, then install them:

```bash
cpx hooks install
```

## Flawfinder Analysis

```bash
# Basic scan
cpx flawfinder

# HTML report
cpx flawfinder --html --output report.html

# CSV output with dataflow analysis
cpx flawfinder --csv --output report.csv --dataflow
```

## Cppcheck Static Analysis

```bash
# Full analysis
cpx cppcheck

# XML report
cpx cppcheck --xml --output report.xml

# Specific checks only
cpx cppcheck --enable style,performance
```

## Sanitizer Checks

```bash
# AddressSanitizer (memory errors)
cpx check --asan

# ThreadSanitizer (data races)
cpx check --tsan

# MemorySanitizer (uninitialized memory)
cpx check --msan

# UndefinedBehaviorSanitizer
cpx check --ubsan
```

