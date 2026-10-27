package main

import (
	"os"
	"os/signal"

	"github.com/gdamore/tcell/v2"
)

// EventInterrupt is posted when the process receives an interrupt: on
// Windows the console's Ctrl+C control event, elsewhere SIGINT. A program
// run from the command line may switch the shared console to processed
// input, after which Ctrl+C no longer arrives as a key but as this event;
// without a handler the Go runtime would end NEAR on it.
type EventInterrupt struct{ tcell.EventTime }

// watchInterrupts turns interrupts into EventInterrupt on the screen.
func watchInterrupts(s tcell.Screen) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
			ev := &EventInterrupt{}
			ev.SetEventNow()
			_ = s.PostEvent(ev)
		}
	}()
}
