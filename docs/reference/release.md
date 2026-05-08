# Release Reference

`terminos` uses Task for local release builds and GitHub Actions for tag releases.

Build local artifacts:

```sh
task release:local VERSION=0.1.0
task release:check VERSION=0.1.0
```

Artifacts are written to `dist/release/`:

```text
terminos_0.1.0_linux_amd64
terminos_0.1.0_linux_arm64
terminos_0.1.0_darwin_amd64
terminos_0.1.0_darwin_arm64
terminos_0.1.0_windows_amd64.exe
SHA256SUMS
```

Create a GitHub release by pushing a version tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds the same artifacts with `task release:local`, verifies them with `task release:check`, and publishes them to the GitHub release.

Version injection uses Go linker flags:

```sh
go build -ldflags "-X github.com/nccurry/terminos/internal/model.ToolVersion=0.1.0"
```
