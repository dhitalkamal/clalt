// Package crew runs Kiro Crew gateways, one per clalt account. it only ever
// touches a gateway that clalt started itself, which it knows from the records
// it keeps under ~/.config/clalt/crew. see docs/plans/crew.md for what was
// checked about Kiro Crew and why each default is what it is.
package crew

// the ports a gateway can get, first to last.
const (
	FirstPort = 5500
	LastPort  = 5599
)

// the file names of one gateway's folder, see Paths.
const (
	recordFile  = "gateway.json"
	logFile     = "gateway.out"
	prevLogFile = "gateway.out.prev"
	localConfig = "config.local.json"
)
