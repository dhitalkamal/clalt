package cli

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/dhitalkamal/clalt/internal/account"
	"github.com/dhitalkamal/clalt/internal/backup"
	"github.com/dhitalkamal/clalt/internal/credential"
)

func (a *App) cmdExport(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: clalt export <file>")
	}
	path := args[0]

	accts, err := a.Store.List()
	if err != nil {
		return err
	}
	if len(accts) == 0 {
		return errors.New("no accounts to export")
	}
	bundle := backup.Bundle{Accounts: map[string]backup.Item{}}
	for _, ac := range accts {
		cred, meta, gerr := a.Store.Get(ac.Name)
		if gerr != nil {
			continue
		}
		credRaw, _ := json.Marshal(cred)
		metaRaw, _ := json.Marshal(meta)
		bundle.Accounts[ac.Name] = backup.Item{Credential: credRaw, Meta: metaRaw}
	}
	plain, err := bundle.Marshal()
	if err != nil {
		return err
	}

	pass, err := a.promptPassword("passphrase to encrypt the export: ")
	if err != nil {
		return err
	}
	confirm, err := a.promptPassword("confirm passphrase: ")
	if err != nil {
		return err
	}
	if pass == "" || pass != confirm {
		return errors.New("passphrases empty or do not match")
	}

	sealed, err := backup.Seal(plain, pass)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, sealed, 0o600); err != nil {
		return err
	}
	a.printf("exported %d account(s) to %s\n", len(bundle.Accounts), path)
	return nil
}

func (a *App) cmdImport(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: clalt import <file>")
	}
	sealed, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	pass, err := a.promptPassword("passphrase to decrypt the export: ")
	if err != nil {
		return err
	}
	plain, err := backup.Open(sealed, pass)
	if err != nil {
		return err
	}
	bundle, err := backup.ParseBundle(plain)
	if err != nil {
		return err
	}

	imported := 0
	for name, item := range bundle.Accounts {
		var cred credential.Credential
		var meta account.Meta
		if uerr := json.Unmarshal(item.Credential, &cred); uerr != nil {
			continue
		}
		_ = json.Unmarshal(item.Meta, &meta)
		if serr := a.Store.Save(name, cred, meta); serr != nil {
			return serr
		}
		imported++
	}
	a.printf("imported %d account(s) from %s\n", imported, args[0])
	return nil
}
