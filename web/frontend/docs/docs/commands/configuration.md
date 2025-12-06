# Configuration Commands

Commands for managing Cpx configuration.

## cpx config set-vcpkg-root

Set the vcpkg installation directory.

```bash
cpx config set-vcpkg-root <path>
```

## cpx config get-vcpkg-root

Get the current vcpkg root directory.

```bash
cpx config get-vcpkg-root
```

## cpx hooks install

Install git hooks using the choices captured during `cpx new`.

```bash
cpx hooks install
```

This command:
- Installs pre-commit / pre-push hooks with the checks you selected in the TUI
- Falls back to defaults (fmt + lint on pre-commit, test on pre-push) if no checks were chosen

