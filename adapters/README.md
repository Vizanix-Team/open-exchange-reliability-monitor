# adapters/

This directory holds cross-exchange adapter *reference material* — captured
fixtures, protocol notes, and links to the official documentation each
adapter was verified against — separate from the adapter implementation
code itself, which lives in `probe/internal/adapter/<venue>/`.

Why split docs from code: reviewers (including exchange engineers checking
our claims about their API) should be able to audit "what did the adapter
author actually read" without needing to read Go. See
`docs/EXCHANGE_ADAPTERS.md` for the adapter interface and the per-venue
implementation notes.

Currently: Bybit's notes are folded directly into
`docs/EXCHANGE_ADAPTERS.md` since it's the only implemented adapter so far.
As more adapters land, expect this directory to hold one subfolder per
venue with captured raw traffic samples used to build
`probe/internal/adapter/<venue>/testdata/` fixtures.
