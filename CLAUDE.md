# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`digital.vasic.discovery` is a standalone Go module for network and service discovery. It provides interfaces and implementations for scanning networks to find services, with an initial focus on SMB/CIFS share discovery via TCP port probing.

## Commands

```bash
# Run all tests
go test ./... -count=1

# Run tests with verbose output
go test -v ./... -count=1

# Run a single package's tests
go test -v ./pkg/scanner/ -count=1
go test -v ./pkg/smb/ -count=1
go test -v ./pkg/report/ -count=1

# Run a specific test
go test -v -run TestNewScanner_NilConfig ./pkg/smb/

# Build all packages
go build ./...

# Tidy dependencies
go mod tidy
```

## Architecture

The module is organized into three packages under `pkg/`:

- **`pkg/scanner`** -- Core interfaces and types. Defines `Scanner` (interface), `Service` (discovered service), and `Config` (scanner configuration). All scanner implementations must satisfy the `Scanner` interface.

- **`pkg/smb`** -- SMB/CIFS discovery scanner. Implements `scanner.Scanner` by attempting TCP connections to SMB ports (445, 139) across a CIDR network range. Uses concurrent goroutines with a semaphore for controlled parallelism. Includes CIDR expansion utilities.

- **`pkg/report`** -- Report generation. Takes scan results and produces structured reports with JSON serialization and human-readable summaries. Groups discovered services by protocol.

### Adding a New Protocol Scanner

1. Create a new package under `pkg/` (e.g., `pkg/ftp/`).
2. Implement the `scanner.Scanner` interface: `Scan()`, `ScanHost()`, `Protocol()`.
3. Set protocol-appropriate default ports in the constructor.
4. Add tests covering: nil config, custom config, unreachable hosts, live listeners, context cancellation.

## Conventions

- **Go**: Constructor injection via `NewXxx(cfg)`, table-driven tests, `*_test.go` beside source files.
- **Error handling**: Wrap errors with `fmt.Errorf` and `%w` for unwrapping.
- **Concurrency**: Use semaphore channels for limiting goroutines; respect `context.Context` cancellation.
- **Testing**: Use `github.com/stretchr/testify` for assertions. Spin up local TCP listeners for integration-style tests.

## Constraints

- No external dependencies beyond `stretchr/testify` for testing.
- No SMB protocol-level libraries; discovery is pure TCP port scanning.
- Context cancellation must be respected in all scan operations.


## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**



## Universal Mandatory Constraints

These rules are inherited from the cross-project Universal Mandatory Development Constraints (canonical source: `/tmp/UNIVERSAL_MANDATORY_RULES.md`, derived from the HelixAgent root `CLAUDE.md`). They are non-negotiable across every project, submodule, and sibling repository. Project-specific addenda are welcome but cannot weaken or override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline. No Git hooks either. All builds and tests run manually or via Makefile / script targets.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`, `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule operations. Including for public repos. SSH keys are configured on every service.
3. **NO manual container commands.** Container orchestration is owned by the project's binary / orchestrator (e.g. `make build` → `./bin/<app>`). Direct `docker`/`podman start|stop|rm` and `docker-compose up|down` are prohibited as workflows. The orchestrator reads its configured `.env` and brings up everything.

### Mandatory Development Standards

1. **100% Test Coverage.** Every component MUST have unit, integration, E2E, automation, security/penetration, and benchmark tests. No false positives. Mocks/stubs ONLY in unit tests; all other test types use real data and live services.
2. **Challenge Coverage.** Every component MUST have Challenge scripts (`./challenges/scripts/`) validating real-life use cases. No false success — validate actual behavior, not return codes.
3. **Real Data.** Beyond unit tests, all components MUST use actual API calls, real databases, live services. No simulated success. Fallback chains tested with actual failures.
4. **Health & Observability.** Every service MUST expose health endpoints. Circuit breakers for all external dependencies. Prometheus / OpenTelemetry integration where applicable.
5. **Documentation & Quality.** Update `CLAUDE.md`, `AGENTS.md`, and relevant docs alongside code changes. Pass language-appropriate format/lint/security gates. Conventional Commits: `<type>(<scope>): <description>`.
6. **Validation Before Release.** Pass the project's full validation suite (`make ci-validate-all`-equivalent) plus all challenges (`./challenges/scripts/run_all_challenges.sh`).
7. **No Mocks or Stubs in Production.** Mocks, stubs, fakes, placeholder classes, TODO implementations are STRICTLY FORBIDDEN in production code. All production code is fully functional with real integrations. Only unit tests may use mocks/stubs.
8. **Comprehensive Verification.** Every fix MUST be verified from all angles: runtime testing (actual HTTP requests / real CLI invocations), compile verification, code structure checks, dependency existence checks, backward compatibility, and no false positives in tests or challenges. Grep-only validation is NEVER sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL).** ALL test and challenge execution MUST be strictly limited to 30-40% of host system resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`, `-p 1` for `go test`. Container limits required. The host runs mission-critical processes — exceeding limits causes system crashes.
10. **Bugfix Documentation.** All bug fixes MUST be documented in `docs/issues/fixed/BUGFIXES.md` (or the project's equivalent) with root cause analysis, affected files, fix description, and a link to the verification test/challenge.
11. **Real Infrastructure for All Non-Unit Tests.** Mocks/fakes/stubs/placeholders MAY be used ONLY in unit tests (files ending `_test.go` run under `go test -short`, equivalent for other languages). ALL other test types — integration, E2E, functional, security, stress, chaos, challenge, benchmark, runtime verification — MUST execute against the REAL running system with REAL containers, REAL databases, REAL services, and REAL HTTP calls. Non-unit tests that cannot connect to real services MUST skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY).** Every reported error, defect, or unexpected behavior MUST be reproduced by a Challenge script BEFORE any fix is attempted. Sequence: (1) Write the Challenge first. (2) Run it; confirm fail (it reproduces the bug). (3) Then write the fix. (4) Re-run; confirm pass. (5) Commit Challenge + fix together. The Challenge becomes the regression guard for that bug forever.
13. **Concurrent-Safe Containers (Go-specific, where applicable).** Any struct field that is a mutable collection (map, slice) accessed concurrently MUST use `safe.Store[K,V]` / `safe.Slice[T]` from `digital.vasic.concurrency/pkg/safe` (or the project's equivalent primitives). Bare `sync.Mutex + map/slice` combinations are prohibited for new code.

### Definition of Done (universal)

A change is NOT done because code compiles and tests pass. "Done" requires pasted terminal output from a real run, produced in the same session as the change.

- **No self-certification.** Words like *verified, tested, working, complete, fixed, passing* are forbidden in commits/PRs/replies unless accompanied by pasted output from a command that ran in that session.
- **Demo before code.** Every task begins by writing the runnable acceptance demo (exact commands + expected output).
- **Real system, every time.** Demos run against real artifacts.
- **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `describe.skip` without a trailing `SKIP-OK: #<ticket>` comment break validation.
- **Evidence in the PR.** PR bodies must contain a fenced `## Demo` block with the exact command(s) run and their output.
