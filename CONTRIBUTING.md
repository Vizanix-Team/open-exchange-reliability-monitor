# Contributing

Thanks for considering a contribution. This project's credibility rests on
measurement discipline, so contributions are held to a higher bar around
**never inventing exchange behavior** and **never letting core code become
exchange-specific**.

## Ground rules

- **Never guess an exchange's API.** If you're implementing or modifying an
  adapter, cite the exact official documentation URL(s) you used in your
  PR description. If something can't be verified, leave a
  `// TODO(verify): ...` and a stub, not an invented value.
- **Core code stays exchange-agnostic.** If you find yourself writing
  `if exchange == "bybit"` outside `probe/internal/adapter/bybit`, that
  logic belongs in the adapter, not in the shared engine.
- **No silent exception swallowing.** Every failure path must classify into
  the error taxonomy (`docs/METRICS.md`) and be observable (metric, log, or
  both) — not just caught and dropped.
- **No unbounded retry loops.** Any retry logic needs a ceiling and backoff.
- **No new unbounded-cardinality labels or fields.** Adding a Prometheus
  label or a ClickHouse partition/order-by key that scales with something
  unbounded (raw error text, per-message IDs, arbitrary symbols) will be
  asked to move into the structured event payload instead.
- **Language discipline in anything user-facing** (dashboard copy, incident
  templates, docs, PR descriptions of new features): no exchange ranking
  language, no "Exchange X is unreliable/best/worst," no composite scores.
  See `docs/METHODOLOGY.md`.

## Adding a new exchange adapter

Follow the checklist in `docs/EXCHANGE_ADAPTERS.md` exactly. In short:
implement `Adapter`, add canonical instrument mapping, add fixtures
(including a deliberate sequence-gap fixture and a malformed-message
fixture), document sequence semantics and quirks, cite your sources.

## Adding a new region

A region is just a probe deployment with its own `node_id`/`region`/
`provider`/`latitude`/`longitude` configuration — no code change needed.
See `deploy/` for an example config. Open a PR adding the region to
`docs/` if you're running it as part of the official probe fleet, so
readers of the dashboard know what geographic coverage exists.

## Adding or changing a metric

Any new metric needs a `docs/METRICS.md` entry (name, unit, type, formula,
caveats) in the same PR as the code that emits it — a metric without a
documented definition will not be merged, per this project's core
transparency requirement.

## Development

```bash
cd probe
go build ./...
go test ./...
```

Tests must not depend on live exchange availability — CI runs against a
fake exchange server (`probe/internal/testutil` / adapter fixtures) so
contributors and CI aren't blocked by real exchange downtime or rate
limits. See `CONTRIBUTING.md`'s testing note and `docs/EXCHANGE_ADAPTERS.md`
for how fixtures are structured.

## Testing expectations for adapter changes

- Unit tests against recorded fixtures (snapshot + delta sequences,
  including at least one gap and one malformed message).
- A reconnect-simulation test if you touch connection lifecycle logic.
- A schema-change test (adapter must classify unrecognized-but-parseable
  new fields as forward-compatible, not fail).

## Pull requests

- Keep PRs scoped: one adapter, one metric, one architectural change at a
  time.
- Reference the exact docs you verified against for any exchange-specific
  claim.
- Be prepared to discuss methodology implications, not just code
  correctness — a technically-correct change that weakens the neutrality or
  reproducibility guarantees above will be pushed back on.

## Code of Conduct

This project follows the Contributor Covenant — see `CODE_OF_CONDUCT.md`.
