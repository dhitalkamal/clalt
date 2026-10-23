package cli

import "strings"

const renameUsage = "clalt rename <old> <new>"

// cmdRename changes the label an account is shown and typed as. nothing else
// moves: claude keys the login to the folder, which keeps its name.
func (a *App) cmdRename(args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return usageError{"unknown flag " + arg, renameUsage}
		}
	}
	if len(args) != 2 {
		return usageError{"give the current label and the new one", renameUsage}
	}
	acc, err := a.findAccount(args[0])
	if err != nil {
		return err
	}
	if err := a.relabel(acc.Name, args[1]); err != nil {
		return err
	}
	a.printf("renamed %s to %s\n", acc.Label(), args[1])
	return nil
}
