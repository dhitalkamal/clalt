package wrapper

import "strings"

// the flags of the first start that a resume does not bring back, by what they
// take. the docs list --mcp-config, --settings, --plugin-dir, --fallback-model
// and --add-dir, and say that bypassing permissions has to be asked for again at
// launch. what a resume does restore (the model, the permission mode the user
// changed to, the agent, the system prompt) is left out on purpose: passing it
// again would undo what was changed in the session.
var (
	carryBool   = map[string]bool{"--strict-mcp-config": true, "--dangerously-skip-permissions": true, "--allow-dangerously-skip-permissions": true}
	carryOne    = map[string]bool{"--settings": true, "--fallback-model": true, "--plugin-dir": true}
	carryMany   = map[string]bool{"--mcp-config": true, "--add-dir": true}
	valueOfFlag = map[string]bool{ // flags that are not carried but take a value, so it is not read as a flag
		"--model": true, "--permission-mode": true, "--agent": true, "--effort": true, "-n": true, "--name": true,
		"--resume": true, "-r": true, "--session-id": true, "--append-system-prompt": true, "--system-prompt": true,
	}
)

// ResumeArgs is the claude command line that continues the session: --resume with
// its id, and the flags of the first start that a resume does not restore.
// the first prompt and anything after a double dash is not carried.
func ResumeArgs(original []string, sessionID string) []string {
	out := []string{"--resume", sessionID}
	for i := 0; i < len(original); i++ {
		arg := original[i]
		if arg == "--" {
			break
		}
		name, _, hasValue := strings.Cut(arg, "=")
		switch {
		case carryBool[arg]:
			out = append(out, arg)
		case carryOne[name]:
			if hasValue {
				out = append(out, arg)
			} else if i+1 < len(original) && !strings.HasPrefix(original[i+1], "-") {
				out = append(out, arg, original[i+1])
				i++
			}
		case carryMany[name]:
			vals := []string{}
			if hasValue {
				out = append(out, arg)
				continue
			}
			for i+1 < len(original) && !strings.HasPrefix(original[i+1], "-") {
				vals = append(vals, original[i+1])
				i++
			}
			if len(vals) > 0 {
				out = append(out, arg)
				out = append(out, vals...)
			}
		case name == "--permission-mode":
			switch {
			case hasValue && arg == "--permission-mode=bypassPermissions":
				out = append(out, arg)
			case !hasValue && i+1 < len(original):
				if original[i+1] == "bypassPermissions" {
					out = append(out, arg, original[i+1])
				}
				if !strings.HasPrefix(original[i+1], "-") {
					i++
				}
			}
		case !hasValue && valueOfFlag[arg] && i+1 < len(original) && !strings.HasPrefix(original[i+1], "-"):
			i++
		}
	}
	return out
}
