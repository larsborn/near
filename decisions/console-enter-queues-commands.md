# While a command runs, Enter queues the next one unless the command asked something

Date: 2026-11-10
Status: accepted
Supersedes the stdin half of console-input-and-mouse-copy.md

## Decision
Enter, while a console command is running, puts what was typed in the queue and runs it
when the current command (and anything queued before it) has finished. The console shows
`[queued: …]` and the line goes into the command history. Enter gives the line to the
running command instead when that command is *being talked to*: it stopped mid-line, which
is what a question looks like, or it has already been answered once (`App.cmdFed`, reset
for every command). Ctrl+Enter always does the other one of the two, and the right of the
input row says which Enter will do. Ctrl+D still closes the input, and Ctrl+C still drops
the queue along with the running command.

## Why
Enter used to always feed the running command, so a command typed while something ran was
swallowed by a program that never reads its input — "simply forgotten", as reported. A
shell keeps such type-ahead and runs it afterwards. The two cases cannot be told apart with
certainty, so the rule is a visible default plus an override: a program that asks a question
writes it without a newline and then reads, which is exactly the unfinished line the console
already tracks, and a program being fed deliberately stays in that mode.

## Consequences
A program that reads without asking anything (`sort`, `cat`) needs Ctrl+Enter for its first
line; Enter continues after that. A program that writes progress without a newline looks
like it is asking, so Enter would feed it: the hint says so and Ctrl+Enter queues. An
answer is echoed onto the question's line rather than a new one, as a terminal does, which
also keeps the question visibly open while the next one is being printed.

## Where
`console.go` (`Pending`, `Echo`, `queueTyped`, `sendStdin`, `feeding`), `app.go` running
branch, `cmdline.go` hint, `console_test.go`.
