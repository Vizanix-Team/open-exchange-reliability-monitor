# tests/

Cross-cutting integration and fuzz fixtures that don't belong to a single
Go package live here as the project grows (e.g. multi-adapter replay
scenarios, load-testing scripts). Today, all tests are colocated with the
code they test under `probe/internal/**/*_test.go` — see
`probe/internal/adapter/bybit` for adapter unit tests against recorded
fixtures and an in-process fake exchange WebSocket server, and
`probe/internal/orderbook` for order-book reconstruction tests. This
directory is reserved for when integration tests need to span multiple
packages/services (e.g. once central ingestion exists — see
`ROADMAP.md`).
