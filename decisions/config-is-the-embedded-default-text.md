# The defaults are one embedded TOML text; the user's file is decoded over it

Date: 2026-10-23
Status: accepted

## Decision
`near -defaults` prints the default configuration, which is also what is written on first
start: every value shown is the default, with comments. The user's file is decoded over the
parsed defaults, so `[keys]` entries merge (unlisted actions keep their defaults) while
arrays such as `[[menus]]` and `search_exclude` replace the defaults wholesale. Adding an
action means touching the default text, `doAction` and `actionHelp`. The config directory
is `near` under the platform's user config directory and also holds `state.json`,
`index.gob` and `menus.toml`.

## Why
One source for defaults and documentation; a generated file that explains itself answered
"where are the keyboard shortcuts configured?".

## Consequences
A binding changed in the defaults does not reach an existing user file that lists every
key (see `live-config-not-regenerated.md`).

## Where
`config.go` defaultConfigTextFor and loadConfig, `main.go` -defaults,
`TestDefaultConfigParsesOnEveryPlatform`.
