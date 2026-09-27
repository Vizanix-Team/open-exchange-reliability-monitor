# proto/

Reserved for shared wire/schema definitions once the central ingestion API
and backend exist (see `ROADMAP.md` Phase 1/2) — e.g. the ingestion batch
format for signed probe measurements, and/or the public read API's schema
if we adopt protobuf/gRPC internally alongside the public JSON API.

Nothing is generated from here yet. The authoritative schema today is
`docs/METRICS.md` (the `probe_measurement` and `ws_event` record
definitions) and the Go types in `probe/internal/model`.
