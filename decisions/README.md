# Decisions

One file per decision made about NEAR's behaviour, defaults, storage or naming that
cannot be derived from the code alone. Files are never rewritten: a changed mind gets a
new file and the old one is marked `Status: superseded by <file>`.

Template:

```markdown
# <decision in one line>

Date: YYYY-MM-DD
Status: accepted            (accepted | superseded by <file> | reverted)

## Decision
What was decided, precisely.

## Why
The reasoning, including which FAR behaviour was matched or deliberately not matched.

## Consequences
What follows from it for users and for the code.

## Where
Files, functions, docs that implement or describe it.
```
