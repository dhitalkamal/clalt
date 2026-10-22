package keychain

import (
	"net/url"
	"os"
	"path/filepath"
	"sort"
)

// FileStore is a plaintext file-backed Store (chmod 0600, not encrypted at
// rest). It is the storage engine EncFileStore wraps with AES-256-GCM for the
// production vault; used bare it is only for tests, so do not point it at real
// long-lived secrets.
type FileStore struct{ dir string }

// NewFileStore roots a file-backed store at dir.
func NewFileStore(dir string) *FileStore { return &FileStore{dir: dir} }

// layout: <dir>/<esc(service)>/<esc(account)>, names URL-escaped so spaces and
// slashes (e.g. the service "Claude Code-credentials") map to safe filenames.
func (f *FileStore) svcDir(service string) string {
	return filepath.Join(f.dir, url.PathEscape(service))
}

func (f *FileStore) path(service, account string) string {
	return filepath.Join(f.svcDir(service), url.PathEscape(account))
}

func (f *FileStore) Get(service, account string) ([]byte, error) {
	data, err := os.ReadFile(f.path(service, account))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

func (f *FileStore) Set(service, account string, data []byte) error {
	if err := os.MkdirAll(f.svcDir(service), 0o700); err != nil {
		return err
	}
	return os.WriteFile(f.path(service, account), data, 0o600)
}

func (f *FileStore) Delete(service, account string) error {
	err := os.Remove(f.path(service, account))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (f *FileStore) List(service string) ([]string, error) {
	entries, err := os.ReadDir(f.svcDir(service))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name, uerr := url.PathUnescape(e.Name())
		if uerr != nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
