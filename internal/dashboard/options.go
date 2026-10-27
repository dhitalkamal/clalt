package dashboard

import "time"

// Options tune a dashboard. the zero value is fine: unicode, color, a refresh
// every five seconds, messages that stay three, loads that give up after thirty.
type Options struct {
	ASCII       bool
	NoColor     bool
	Refresh     time.Duration
	FlashFor    time.Duration
	LoadTimeout time.Duration
	// Now is the clock the times in a Snapshot are shown against.
	Now func() time.Time
	// Logger gets the full text of every load and action failure, never
	// transcript text. it runs on the UI goroutine, so it must not block. nil
	// logs nothing.
	Logger func(msg string)
}

func (o Options) withDefaults() Options {
	if o.Refresh <= 0 {
		o.Refresh = 5 * time.Second
	}
	if o.FlashFor <= 0 {
		o.FlashFor = 3 * time.Second
	}
	if o.LoadTimeout <= 0 {
		o.LoadTimeout = 30 * time.Second
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// EnvOptions reads CLALT_ASCII=1 and NO_COLOR, set and not empty, from the
// environment through getenv.
func EnvOptions(getenv func(string) string) Options {
	return Options{
		ASCII:   getenv("CLALT_ASCII") == "1",
		NoColor: getenv("NO_COLOR") != "",
	}
}
