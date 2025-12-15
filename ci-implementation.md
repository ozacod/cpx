targets:
  # MODE 1: PULL (Standard CI)
  # "I want to download a specific image from a registry."
  - name: "ci-ubuntu"
    runner: "docker"
    docker:
      mode: "pull"
      image: "ubuntu:22.04"
      platform: "linux/amd64"
      # Optional: force pull behavior
      pullPolicy: "always"

  # MODE 2: LOCAL (Offline / Custom Toolchain)
  # "I have an image on my machine. Don't touch the network."
  - name: "my-custom-toolchain"
    runner: "docker"
    docker:
      mode: "local"
      image: "my-toolchain"
      platform: "linux/arm64"

  # MODE 3: BUILD (Dockerfile)
  # "Build this Dockerfile before running."
  - name: "ci-dockerfile"
    runner: "docker"
    docker:
      mode: "build"
      # In 'build' mode, 'image' becomes the TAG for the result
      image: "cpx-dev-image"
      platform: "linux/arm64"
      build:
        context: "."
        dockerfile: "dockerfiles/my-ubuntu.Dockerfile"
        args:
          GCC_VER: "13"

  # MODE 3: BUILD (native)
  - name: "local-debug-info"
    runner: "native"
    env:
      CC: "clang"
      CXX: "clang++"




need to hash builds so I can cpx-<name> (hashes image name envs etc) under .cache for subsequent builds etc so I can run with cpx ci run local-dev or cpx ci run dc-linux-arm64 u will keep build debug o3 etc directories for basic builds(which are called with basic commands like cpx run - build etc) but if native builds are defined under cpx.ci their builds will be stored under .cache/ci/cpx-<name>. FlowHere is how cpx should handle a mode: build target now:1 Calculate Hash: Read Dockerfile content + Build Args $\rightarrow$ Hash.2Determine Tag: cpx/<target_name>:<Hash>.3Check Docker: Does this specific tag exist?Yes: Reuse it (Fast start).No: Build it.4Run: Mount .cache/ci/<target_name> (No hash needed here!) and run the container.

The Strategy: "Readable Folder, Hashed Image"We will hash only the Build Configuration (Dockerfile content + Args) to create the Docker Tag.Project A (local-dev): Uses Dockerfile A $\rightarrow$ Image: cpx/local-dev:a1b2c3Project B (local-dev): Uses Dockerfile B $\rightarrow$ Image: cpx/local-dev:x9y8z7This solves the collision problem while allowing two identical projects to share the cached image automatically.
