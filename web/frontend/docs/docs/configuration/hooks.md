# Git Hooks Configuration

Git hooks are configured through the interactive TUI (`cpx new`). There is no `cpx.yaml` to edit—pick the checks you want when creating the project.

## Choose hooks in the TUI

During `cpx new`, select which checks to enforce:
- `fmt` - Format code with clang-format
- `lint` - Run clang-tidy static analysis
- `test` - Run tests (blocking for pre-push)
- `flawfinder` - Run Flawfinder security analysis
- `cppcheck` - Run Cppcheck static analysis
- `check` - Run code check

## Installation

After generation, install the hooks into `.git/hooks`:

```bash
cpx hooks install
```

## Behavior

- If you picked checks in the TUI, those hooks are installed (e.g., `pre-commit`, `pre-push`)
- If you skipped hook selection, cpx installs sensible defaults: fmt + lint on pre-commit, test on pre-push

