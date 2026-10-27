package cli

import "time"

func after(seconds int) <-chan time.Time { return time.After(time.Duration(seconds) * time.Second) }
