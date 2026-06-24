# AGENTS.md

## INHERITED FROM constitution/AGENTS.md

All rules in `constitution/AGENTS.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. See parent root `CLAUDE.md` §6.AD for the Lava-specific incorporation context (29th §6.L cycle, 2026-05-14) and §6.AD-debt for the implementation-gap inventory. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `AGENTS.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

Guidelines for AI agents working with this repository.

## Repository Purpose

This is the `digital.vasic.discovery` Go module -- a standalone library for network and service discovery. It is designed to be imported by other projects (e.g., Catalogizer's catalog-api) to discover services on local networks.

## Key Files

| File | Purpose |
|---|---|
| `pkg/scanner/scanner.go` | Core `Scanner` interface and `Service`/`Config` types |
| `pkg/smb/smb.go` | SMB/CIFS port scanner implementation |
| `pkg/report/report.go` | Scan report generation (JSON + text) |
| `go.mod` | Module definition: `digital.vasic.discovery` |

## How to Verify Changes

After any modification, always run:

```bash
go mod tidy && go build ./... && go test ./... -count=1
```

All tests must pass. There are no external service dependencies for testing; SMB tests use local TCP listeners.

## Adding New Scanner Implementations

When adding support for a new protocol:

1. Create `pkg/<protocol>/<protocol>.go` implementing `scanner.Scanner`.
2. Create `pkg/<protocol>/<protocol>_test.go` with comprehensive tests.
3. Default ports should be set in the constructor when the config has none.
4. The `Protocol()` method must return a lowercase string identifier.
5. All scan methods must honor `context.Context` cancellation.
6. Use the concurrency pattern from `pkg/smb/smb.go` (semaphore + WaitGroup).

## Do Not

- Add external dependencies beyond `stretchr/testify` without explicit approval.
- Modify the `scanner.Scanner` interface without updating all implementations.
- Skip context cancellation checks in scan loops.
- Create GitHub Actions workflow files.
