# Stop regenerating the user's live config; check for a lost edit

## What
During development the live config `%APPDATA%\near\config.toml` has been overwritten with
`near -defaults` every time a default key binding changed, after checking it was still the
untouched generated file. On 2026-10-24 the pre-check found one line in the live file that the
defaults did not have (an apparent user edit) and the file was overwritten anyway. The user
was asked which setting it was and has not answered yet.

## Why it matters
Users will edit this file. Overwriting it loses their settings; the merge behaviour of the
config loader (`[keys]` map merges, unlisted actions keep defaults) means regeneration is not
even needed for new keys to work.

## Status
- Defaults are the TOML text in `defaultConfigTextFor` (`config.go`); the loader decodes the
  user's file over the parsed defaults, so new actions get their default keys automatically.
- Only *renamed/moved* defaults (e.g. a key reassigned from one action to another) can
  collide with an old generated file, because the old file still lists the old assignment.

## Options
1. Never touch the live file again; when a default key moves, mention it in the commit/README
   and rely on `near -defaults` for users who want the new file.
2. Add `near -upgrade-config`: rewrite the file keeping user-changed values (diff against the
   defaults at the time the file was generated — would need a version stamp in the file).
3. Warn at startup when the config binds a key that the defaults now use for something else.

## Acceptance
- Ask the user (or diff against `config.toml.bak` and memory) whether a setting was lost and
  restore it.
- Decide on option 1, 2 or 3, implement, and remove the "regenerate" habit from CLAUDE.md.
