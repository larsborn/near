# Every finished piece of work is committed and pushed; releases are cut only when asked

Date: 2026-10-24
Status: accepted

## Decision
After each piece of work: gofmt, vet, tests, commit with a message that says what changed
for the user, push. A release (tag `vX.Y.Z`, which the release workflow turns into GitHub
release archives) is made only on request, and only after CI is green on the commit.

## Why
The user asked for commit-and-push to be automatic and for releases to stay a deliberate
act. CI was red for five pushes once; tagging a red commit would have shipped failing tests.

## Where
`CLAUDE.md` "Git workflow", `.github/workflows/release.yml`.
