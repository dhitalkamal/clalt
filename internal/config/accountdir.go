package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// windowsDevices are names Windows treats as devices whatever the extension.
var windowsDevices = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// AccountDirFor returns the folder of account id after checking that the id is
// one local path element: not empty, not "." or "..", no separator, no NUL and
// no Windows device name. every id that becomes a folder name goes through it.
// it is looser than account.ValidateName on purpose, so a legacy id from
// v0.1.0 that is still safe as a folder name keeps working.
func AccountDirFor(id string) (string, error) {
	if err := checkPathElement(id); err != nil {
		return "", fmt.Errorf("account id %q: %w", id, err)
	}
	root := AccountsDir()
	if !filepath.IsAbs(root) {
		return "", errors.New("clalt's folder is not an absolute path (is HOME set?)")
	}
	return accountDir(id), nil
}

func checkPathElement(name string) error {
	switch {
	case name == "":
		return errors.New("is empty")
	case name == "." || name == "..":
		return errors.New("is not a folder name")
	case strings.ContainsAny(name, `/\`):
		return errors.New("contains a path separator")
	case strings.ContainsRune(name, 0):
		return errors.New("contains a NUL byte")
	}
	stem, _, _ := strings.Cut(name, ".")
	if windowsDevices[strings.ToLower(stem)] {
		return errors.New("is a reserved device name on Windows")
	}
	return nil
}
