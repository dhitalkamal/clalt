package crew

// State is where a gateway stands.
type State string

// the states of a gateway. running is a process clalt started and has checked
// is still that gateway. stale is a record whose process is gone, or whose pid
// belongs to another process now. unverified is a process that is alive and
// started when the gateway did, but that clalt cannot confirm is the gateway.
// stopped is an account with no record, and broken is a record that cannot be
// read.
const (
	StateRunning    State = "running"
	StateStale      State = "stale"
	StateUnverified State = "unverified"
	StateStopped    State = "stopped"
	StateBroken     State = "broken"
)
