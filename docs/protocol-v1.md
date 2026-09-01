# Error-directed evolution planner protocol v1

## Authority boundary

The `.gooo` contract owns the fixed denominator, status vocabulary, precedence,
unknown tuple, allowed delta operations, deterministic candidate order, and
generation plan. Go parses those declarations, executes the candidate search,
calls the supplied released compiler oracle, and emits artifacts.

Terminal records and minimized counterexamples are immutable inputs. Their
declared scenario, source digest, toolchain digest, baseline phase digest, and
asset file digests must agree before a candidate can close. A digest mismatch
is `REFUTED`; missing evidence is `UNKNOWN`.

All generated phase text and JSON are written below the caller-owned output
directory. The input repository is never edited, and `repository_writes` and
`merge_authority` remain exact integer zeroes in every bundle.

## Candidate selection

The planner enumerates the four operations in the `.gooo` order. A candidate is
valid only when the compiler oracle accepts its phase and the immutable repair
record accepts that operation, activity cardinality, localization stage count,
and expected decision/reason. It then chooses the lowest declared cost. Equal
minimum candidates produce `UNKNOWN` with all six fields instead of an
arbitrary tie-break.

The known four-activity case uses one `SPLIT`: `NormalizeSource` is retired,
`ParseSource` and `ValidateStableIDs` are added, and the typed edges are
rewired. The released compiler v0.3.0 oracle accepts the resulting four-role
phase. The bundle contains the operation-level changes and the inverse
rollback receipt; it is not a source patch or an adoption transaction.

## Decisions and improvement

`REFUTED` dominates `UNKNOWN`, which dominates `CLOSED`. Each UNKNOWN carries
`stage`, `step`, `reason`, `unknown_class`, `next_operation`, and `blocked_by`.
The ordinary `improvement` field remains `UNKNOWN` unless a matched prior
report supplies the same scenario, source, contract, and toolchain identities
plus exact integer before/after values. No percentage, score, or inferred
ranking is emitted. The separate scenario self-improvement field can close
only after same-counterexample resolution and guardrail non-regression.

