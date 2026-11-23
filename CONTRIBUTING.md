# Contributing to LocalGo

Thanks for helping improve LocalGo. Contributions are welcome through issues and pull requests.

## Before You Start

Please check the existing issues and pull requests before starting substantial work. For larger changes, open an issue first so the proposed behavior and scope can be discussed.

LocalGo is a Go implementation of the LocalSend v2.2 protocol. The most useful project background is in:

- [Code Walkthrough](docs/CODE_WALKTHROUGH.md) for the package layout and request flow.
- [Library Guide](docs/LIBRARY_GUIDE.md) for using LocalGo as a Go library.
- [Protocol documentation](protocol/README.md) for protocol-specific behavior.

## Development Requirements

- Go `1.26.3` or a compatible Go 1.26 toolchain.
- Git.
- `golangci-lint` for the full local lint check.
- A working network environment for tests that exercise discovery or transfers.

The CI workflow tests on Linux, macOS, and Windows. Keep platform-specific behavior behind the existing build-tagged files and verify changes on every affected platform when possible.

## Get the Code

For a fork-based workflow:

```bash
git clone https://github.com/<your-user>/localgo.git
cd localgo
git remote add upstream https://github.com/bethropolis/localgo.git
git fetch upstream
```

Create feature branches from `upstream/dev`:

```bash
git switch -c my-change upstream/dev
```

Pull requests should target `dev` unless the change is specifically requested for a release or maintenance branch.

## Local Checks

Install or synchronize dependencies:

```bash
go mod download
go mod tidy
git diff --exit-code
```

Run the focused checks while developing:

```bash
go fmt ./...
go vet ./...
go test -race -timeout 60s ./...
go build ./cmd/localgo
```

The corresponding Make targets are:

```bash
make fmt       # Format Go files
make vet       # Run go vet
make lint      # Run golangci-lint
make test      # Run race-enabled tests
make build     # Build the release-style binary
make check     # Format, vet, and lint
make ci        # Dependencies, checks, tests, and build
```

Run the relevant package tests first for a fast feedback loop, then run `make ci` before opening a pull request. Do not commit generated coverage output, local binaries, or distribution artifacts.

## Making Changes

- Keep changes focused and consistent with existing package boundaries.
- Prefer standard library and existing project helpers over new dependencies.
- Preserve support for Linux, macOS, Windows, and Android/Termux where applicable.
- Add or update tests for behavior changes, especially protocol, discovery, TLS, storage, and cross-platform code.
- Use `go fmt` and keep exported APIs documented.
- Do not include secrets, personal configuration, generated files, or unrelated formatting changes.
- Update user-facing documentation when commands, configuration, deployment, or protocol behavior changes.

For network and TLS changes, include tests that cover both successful and failure paths. Avoid weakening certificate verification or authentication solely to make a test pass.

## Pull Requests

A good pull request should:

- Explain the user or developer problem being solved.
- Describe the implementation and any compatibility considerations.
- Include tests and the commands used to run them.
- Call out platform-specific behavior, migration concerns, or follow-up work.
- Keep the title concise and action-oriented, for example: `fix: handle TLS client certificates during discovery`.

Keep each pull request reviewable. Split unrelated refactors or documentation changes into separate pull requests when practical.

CI must pass before merging. Reviewers may request changes to tests, documentation, API design, security, or cross-platform behavior.

## Commit Messages

Use short, descriptive commit subjects with a conventional prefix when appropriate:

```text
feat: add a discovery option
fix: preserve file timestamps during transfer
docs: clarify container networking
test: cover TLS client authentication
```

Keep the subject in the imperative mood and explain important implementation or compatibility details in the body.

## Reporting Security Issues

Do not disclose an unpatched security issue in a public issue or pull request. Contact the project maintainers privately through the repository's available GitHub security contact channels.

## License

By contributing, you agree that your contribution is provided under the project's [MIT License](LICENSE).
