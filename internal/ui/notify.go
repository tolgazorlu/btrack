package ui

import "github.com/tolgazorlu/btrack/internal/notify"

// Notify and Bell are thin shims for backward compatibility. New code
// should import internal/notify directly.
func Notify(title, body string) { notify.Notify(title, body) }
func Bell()                     { notify.Bell() }
