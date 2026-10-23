package cli

// inForegroundOfTerminal is always no here: there are no process groups to ask,
// so only the input decides.
func inForegroundOfTerminal() bool { return false }
