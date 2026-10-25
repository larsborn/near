# cd on the command line expands ~, $VAR, ${VAR} and %VAR%

Date: 2026-10-25
Status: accepted

## Decision
`cd <dir>` moves the active panel; the argument goes through expandPath, which expands a
leading `~` and environment variables in both the Unix and the Windows notation before
the path is resolved. Paths in the config (`search_roots`, menu `cwd`) get the same
expansion.

## Why
"`cd %APPDATA%` should work."

## Where
`config.go` expandPath, `cmdline.go` handleCd.
