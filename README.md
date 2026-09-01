# gooo-error-directed-evolution-planner

`gooo-error-directed-evolution-planner` consumes two immutable evidence
assets: an explanation-carrying terminal record from
[`gooo-reflexive-compiler-slice@v0.3.0`](https://github.com/kimjooyoon/gooo-reflexive-compiler-slice/releases/tag/v0.3.0)
and a minimized counterexample from
[`gooo-semantic-counterexample-reducer@v0.1.1`](https://github.com/kimjooyoon/gooo-semantic-counterexample-reducer/releases/tag/v0.1.1).
It emits the smallest deterministic candidate allowed by
[`.gooo/evolution-planner.gooo`](.gooo/evolution-planner.gooo): `ADD`,
`RETIRE`, `SPLIT`, or `REWIRE`.

The planner is candidate-only. It never applies a candidate to the input
repository, opens or merges a pull request, or writes outside a caller-owned
output directory. The candidate phase is evaluated in that temporary output
with the released compiler's phase oracle. Every output includes a candidate
bundle, causal rationale, and exact rollback receipt.

## Run

The first argument is a command. `manifest` reports the `.gooo` contract;
`plan` emits one scenario; `conformance` runs the fixed five-scenario corpus.
The `--compiler` value must be the immutable compiler oracle executable.

```text
go run ./cmd/gooo-error-directed-evolution-planner conformance \
  --contract .gooo/evolution-planner.gooo \
  --repo-root . \
  --compiler /path/to/gooo-reflexive-compiler-slice \
  --out /caller-owned/temp/planner
```

Outputs are `candidate-bundle.json`, `causal-rationale.json`,
`rollback-receipt.json`, `candidate-phase.gooo`, and `planner-report.json`.
No local test, build, vet, or conformance result is a success authority;
GitHub Actions is the executable verification environment.

## Fixed corpus

The denominator is exactly five scenarios:

| Scenario | Expected state | Purpose |
| --- | --- | --- |
| `known-4-activity` | `CLOSED` | minimum `SPLIT` candidate closes the known four-activity counterexample |
| `insufficient-evidence` | `UNKNOWN` | preserves the terminal record's six-field UNKNOWN tuple |
| `reason-changing-candidate` | `REFUTED` | rejects a candidate whose oracle reason changes |
| `ambiguous-equal-candidates` | `UNKNOWN` | refuses two equal-cost candidates without disambiguating evidence |
| `replay-closed-corpus` | `CLOSED` | replays the CLOSED/UNKNOWN/REFUTED guardrail corpus without regression |

Resolution is always `REFUTED > UNKNOWN > CLOSED`. An improvement claim is
`UNKNOWN` until the same scenario, source, contract, and Go toolchain have an
exact integer before/after pair. The scenario-level self-improvement claim is
`CLOSED` only when the same counterexample is resolved and every guardrail
case remains unchanged.

## Provenance and release

The bootstrap main commit is the only `BOOTSTRAP_EXCEPTION` and contains only
`.gitignore`, `LICENSE`, and `README.md`. The lock is recorded in
[`contracts/bootstrap-lock-v1.json`](contracts/bootstrap-lock-v1.json).
Post-bootstrap implementation is delivered through one open pull request at
most. External Gooo components are consumed only through immutable release
tags and asset digests recorded in
[`contracts/upstream-lock-v1.json`](contracts/upstream-lock-v1.json).

CI records exact integer inventory and execution evidence: Go/Gooo physical
lines and file counts, regular files, subdirectories, generated artifact
count/bytes, peak RSS, compile/build/test/conformance/integration milliseconds,
and total/selected/executed/reused/failed/unknown tests. The root README is
excluded from inventory. Release workflows refuse existing tags/releases and
never delete or overwrite failed or non-immutable historical releases.

See [`docs/protocol-v1.md`](docs/protocol-v1.md) for the boundary and
[`docs/release-policy.md`](docs/release-policy.md) for the lifecycle.
