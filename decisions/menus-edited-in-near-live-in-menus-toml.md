# Menus edited inside NEAR are stored in menus.toml, which then replaces config.toml's menus

Date: 2026-10-25
Status: accepted

## Decision
Ins, Del, F4, Alt+F4, Ctrl+Up and Ctrl+Down inside a menu edit it like FAR's user menu.
The first such edit writes all menus to `menus.toml` next to the config; from then on
`loadConfig` takes the menus from that file and ignores the `[[menus]]` of config.toml.
config.toml itself is never rewritten. Menus are referred to by name during edits because
creating a submenu reallocates the menus slice.

## Why
Rewriting config.toml through the TOML encoder would lose the user's comments and layout.
FAR also keeps its user menu in a separate file (FarMenu.ini). Merging two sources would be
confusing; one file wins.

## Consequences
After the first in-app edit, menu changes made in config.toml have no effect until
menus.toml is deleted. The menu list's hint names the file in use.

## Where
`usermenu.go`, `config.go` loadConfig, config comment above `[[menus]]`, README.
