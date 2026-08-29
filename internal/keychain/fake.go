package keychain

import (
	"sort"
	"sync"
)

// Fake is an in-memory Store for tests. It never touches the real keychain.
type Fake struct {
	mu   sync.Mutex
	data map[string]map[string][]byte // service -> account -> data
}

// NewFake returns an empty in-memory store.
func NewFake() *Fake {
	return &Fake{data: map[string]map[string][]byte{}}
}

func (f *Fake) Get(service, account string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	svc, ok := f.data[service]
	if !ok {
		return nil, ErrNotFound
	}
	v, ok := svc[account]
	if !ok {
		return nil, ErrNotFound
	}
	// return a copy so callers cannot mutate stored bytes
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (f *Fake) Set(service, account string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.data[service] == nil {
		f.data[service] = map[string][]byte{}
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	f.data[service][account] = cp
	return nil
}

func (f *Fake) Delete(service, account string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if svc, ok := f.data[service]; ok {
		delete(svc, account)
	}
	return nil
}

func (f *Fake) List(service string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	svc, ok := f.data[service]
	if !ok {
		return nil, nil
	}
	names := make([]string, 0, len(svc))
	for k := range svc {
		names = append(names, k)
	}
	sort.Strings(names)
	return names, nil
}
