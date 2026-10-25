# The developer's live config is edited surgically, never regenerated

Date: 2026-10-25
Status: accepted

## Decision
`%APPDATA%\near\config.toml` is the user's file. When a default key binding or default
value changes, only the affected line is changed in that file (on request), and the user
is told. The file is not replaced by `near -defaults` output any more.

## Why
Regenerating once overwrote a possible user edit (`todo/live-config-regeneration-policy.md`).

## Consequences
The live file drifts from the generated default text; a new default only reaches it when
the matching line is updated by hand.

## Where
`CLAUDE.md` "Commands" paragraph on the live config, `todo/live-config-regeneration-policy.md`.
