# Gooo two-generation bootstrap

This repository is a deliberately small, executable self-hosting slice for
Gooo. The authoritative meaning is in
[`.gooo/two-generation.gooo`](.gooo/two-generation.gooo): it declares the
three parser rules, normalized semantic IR, deterministic Go emitter, two-stage
generation plan, corpus denominator, and `REFUTED > UNKNOWN > CLOSED`
resolution policy.

The trusted stage0 command reads those rules and a `.gooo` input, emits a
standalone stage1 Go executor, and records stage1's normalized IR plus the
canonical generated-artifact digest. The generated stage1 executor consumes the
exact same input bytes and emits stage2. CI compares both normalized IR
digests and both generated-artifact digests. Source text equality alone is not
the closure condition.

The six-cell parser corpus contains two `CLOSED`, two `UNKNOWN`, and two
`REFUTED` cases. Unknown results preserve `stage`, `step`, `reason`,
`unknown_class`, `next_operation`, and `blocked_by`. A fixed-point mismatch is
reported at the first differing IR path; missing evidence is `UNKNOWN`.

Only the self-hosting slice is evaluated. This is not a whole-language,
product-value, performance, or global self-improvement claim. Integer
improvement claims require the same scenario, source, contract, and toolchain
digests plus an integer before/after pair; otherwise the result remains
`UNKNOWN`.

All generated files and reports are written to caller-owned temporary output
directories. The root `README.md` is excluded from inventory measurements.
GitHub Actions is the verification authority and records exact inventory,
runtime, test, conformance, integration, artifact, and digest evidence.
