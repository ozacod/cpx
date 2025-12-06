# CI/CD Setup

Set up continuous integration and cross-compilation for your project.

## 1. Download Dockerfiles

```bash
cpx upgrade
```

This downloads Dockerfiles to `~/.config/cpx/dockerfiles/`

## 2. Configure cpx.ci

Edit `cpx.ci` in your project root and add targets:

```yaml
targets:
  - name: linux-amd64
    dockerfile: Dockerfile.linux-amd64
    image: cpx-linux-amd64
    triplet: x64-linux
    platform: linux/amd64
```

The `cpx.ci` file is created automatically with empty targets when you run `cpx new`.

## 3. Generate CI Workflows (Optional)

### GitHub Actions

```bash
cpx ci init --github-actions
```

This creates `.github/workflows/ci.yml`

### GitLab CI

```bash
cpx ci init --gitlab
```

This creates `.gitlab-ci.yml`

These workflow files automatically call `cpx ci` during CI runs.

## 4. Build for Multiple Platforms

```bash
cpx ci
```

Artifacts will be in the `out/` directory.

