package account

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAccountLabelFallsBackToTheId(t *testing.T) {
	if got := (Account{Name: "robin-2", Meta: Meta{Label: "robin"}}).Label(); got != "robin" {
		t.Errorf("Label() = %q, want robin", got)
	}
	if got := (Account{Name: "legacy"}).Label(); got != "legacy" {
		t.Errorf("Label() = %q, want the id when no label is set", got)
	}
}

func TestNewMetaFieldsAreOmittedWhenUnset(t *testing.T) {
	raw, err := json.Marshal(Meta{Email: "a@example.com", AddedAt: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"label", "org", "signedInAt", "hidden"} {
		if _, ok := keys[k]; ok {
			t.Errorf("unset field %q is written: %s", k, raw)
		}
	}
}

func TestNewMetaFieldsUseTheAgreedJSONNames(t *testing.T) {
	at := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(Meta{Label: "robin", Org: "Example Org", SignedInAt: at, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"label", "org", "signedInAt", "hidden"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("field %q is missing from %s", k, raw)
		}
	}
}
