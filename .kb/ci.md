# Preface

This document describes the CI workflow for tlapm-mcp, which runs automated tests on every push and pull request to `main`.

Read the top-level `.kb/agents.md` file before continuing below.

# Overview

The CI pipeline lives in `.github/workflows/ci.yml` and runs on `push` and `pull_request` targeting the `main` branch. It sets up Go 1.24, downloads the `tlapm` binary from GitHub Releases, and runs the full test suite with the race detector.

# Important

- The `tlapm` binary is downloaded fresh each run from the `1.6.0-pre` release (`tlapm-1.6.0-pre-x86_64-linux-gnu.tar.gz`). It is extracted to `$RUNNER_TEMP/tlapm/` and its `bin/` directory is added to `$GITHUB_PATH`.
- Integration tests in `internal/prover/` depend on the `tlapm` binary being available on PATH.
- Go module cache is enabled via `actions/setup-go` to speed up subsequent runs.
- Tests run with `-race -count=1 -v` for race detection, no caching, and verbose output.
