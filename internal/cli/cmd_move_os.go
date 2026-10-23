package cli

import "runtime"

// osCanMove is false on windows, where a process cannot be asked to end.
var osCanMove = runtime.GOOS != "windows"
