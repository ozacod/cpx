# Cross-Compilation with Docker

Cpx supports cross-compilation using Docker containers. The `cpx.ci` file is automatically created when you run `cpx new`.

## cpx.ci Configuration

Example `cpx.ci` file:

```yaml
targets:
  - name: linux-amd64
    dockerfile: Dockerfile.linux-amd64
    image: cpx-linux-amd64
    triplet: x64-linux
    platform: linux/amd64

build:
  type: Release
  optimization: 2
  jobs: 0

output: out
```

## Building for Multiple Platforms

```bash
# Build for all targets in cpx.ci
cpx ci

# Build only for specific target
cpx ci --target linux-amd64

# Rebuild Docker images
cpx ci --rebuild
```

## Artifacts

Build artifacts will be in the `out/` directory after compilation.

