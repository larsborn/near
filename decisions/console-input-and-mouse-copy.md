# Typed text goes to the running command's stdin; a mouse drag over console rows copies on release

Date: 2026-10-28
Status: accepted

## Decision
Console commands get a stdin pipe. While a command runs, printable keys type into the
command line (shown without a prompt), Enter sends the line plus newline to the command and
echoes it in the console, Ctrl+D (and Ctrl+Z) closes the input; Up/Down/Tab/Esc do nothing
meanwhile; Ctrl+C still stops the command. Text is marked by dragging the left mouse button
over console rows, in the full console view or the band under the panels, in reading order;
releasing the button copies the marking to the clipboard and reports the count. The marking
is dropped by any key, a click elsewhere or new output. No keyboard marking mode.

## Why
`git add --patch` printed its prompt and returned at once because stdin was the null device;
the user also wanted to copy output from the shell. Auto-copy on release is how conhost's
QuickEdit and most terminals behave and keeps Ctrl+C free for stopping the command.

## Consequences
stdin is a pipe, not a terminal: programs that check for a terminal behave as in a pipe, and
those that read the console directly (msvcrt.getwch) still compete with NEAR for keys. A line
typed but not sent when the command ends is dropped.

## Where
`console.go` runInConsole/sendStdin/closeStdin, `app.go` running-command key branch,
`cmdline.go` drawCmdLine, `console_select.go`, `mouse.go`, README "Console view".
