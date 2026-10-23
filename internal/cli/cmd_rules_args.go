package cli

import "strings"

const rulesUsage = "clalt rules <name> [--add-folder PATH]... [--remove-folder PATH]... [--auto-switch | --no-auto-switch]"

// rulesRequest is what the arguments of rules ask for: an account, and the
// changes to make to its rules, if any. autoSwitch is nil to leave it as it is.
type rulesRequest struct {
	name        string
	add, remove []string
	autoSwitch  *bool
}

func (r rulesRequest) changes() bool {
	return len(r.add) > 0 || len(r.remove) > 0 || r.autoSwitch != nil
}

// parseRulesArgs reads the arguments. this command writes the file that decides
// where an account may be used, so a flag it does not know, a second name or a
// switch that is both on and off is an error and not a guess.
func parseRulesArgs(args []string) (rulesRequest, error) {
	var req rulesRequest
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--add-folder" || arg == "--remove-folder":
			if i+1 >= len(args) {
				return req, usageError{arg + " needs a folder", rulesUsage}
			}
			i++
			if arg == "--add-folder" {
				req.add = append(req.add, args[i])
			} else {
				req.remove = append(req.remove, args[i])
			}
		case arg == "--auto-switch" || arg == "--no-auto-switch":
			on := arg == "--auto-switch"
			if req.autoSwitch != nil && *req.autoSwitch != on {
				return req, usageError{"--auto-switch and --no-auto-switch both given", rulesUsage}
			}
			req.autoSwitch = &on
		case strings.HasPrefix(arg, "-"):
			return req, usageError{"unknown flag " + arg, rulesUsage}
		case req.name != "":
			return req, usageError{"unexpected argument " + arg, rulesUsage}
		default:
			req.name = arg
		}
	}
	if req.name == "" {
		return req, usageError{"which account? give it a name", rulesUsage}
	}
	return req, nil
}
