# UI tests run headless on a simulation screen and must not depend on machine speed or state

Date: 2026-10-23
Status: accepted

## Decision
Every UI test builds an App on tcell's SimulationScreen with a temporary tree, feeds keys
through handleEvent and asserts on the rendered screen text. Tests never read the
developer's clipboard (`readClip` is nil), never run the real search roots, and slow file
copies with the `copyPace` hook so queue states are observable on CI runners that copy
24 MB in milliseconds. Watcher tests poll with `waitFor`. CI runs the suite on Linux,
Windows and macOS and cross-compiles every release target; a release is tagged only when
CI is green.

## Why
Timing-dependent tests failed five pushes in a row on GitHub's runners; a clipboard test
once saw the developer's clipboard.

## Where
`app_test.go` testApp and helpers, `fsops.go` copyPace, `.github/workflows/ci.yml`,
CLAUDE.md "Testing approach".
