# Creating a Project

Examples of creating different types of projects.

## Basic Executable

```bash
# Launch the TUI and pick "Executable"
cpx new

# When prompted:
# - Name: my_app
# - Project type: Executable
# - Test framework: your choice

# After generation
cd my_app
cpx build
cpx run
```

## Library Project

```bash
# Launch the TUI and pick "Library"
cpx new

# When prompted:
# - Name: my_lib
# - Project type: Library
# - Enable tests if you need them

# After generation
cd my_lib
cpx build
```

## Choose a Test Framework

```bash
# Start the TUI and select the framework you prefer
cpx new

# Options available in the TUI:
# - GoogleTest (default)
# - Catch2
# - doctest
```

