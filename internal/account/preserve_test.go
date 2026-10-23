package account

import (
	"reflect"
	"testing"
	"time"
)

// the old Save replaced a whole Meta, which would have dropped the label, the
// organization, the hidden flag and the sign-in time that it had never heard of.
// nothing replaces a Meta wholesale now, and this test keeps it that way: every
// field is set to something other than its zero value, and each mutator may
// change only the fields it is for.

// fullMeta sets every field of a Meta by reflection, so a field added later is
// covered without anybody having to remember this test.
func fullMeta(t *testing.T) Meta {
	t.Helper()
	var m Meta
	v := reflect.ValueOf(&m).Elem()
	for i := 0; i < v.NumField(); i++ {
		f, name := v.Field(i), v.Type().Field(i).Name
		switch {
		case f.Kind() == reflect.String:
			f.SetString("x-" + name)
		case f.Kind() == reflect.Bool:
			f.SetBool(true)
		case f.Kind() == reflect.Float64:
			f.SetFloat(1.5)
		case f.Type() == reflect.TypeOf(time.Time{}):
			f.Set(reflect.ValueOf(fixedNow.Add(time.Duration(i+1) * time.Hour)))
		default:
			t.Fatalf("Meta.%s has a type this test cannot fill, teach fullMeta about it", name)
		}
	}
	return m
}

func TestNoMutatorReplacesAMetaWholesale(t *testing.T) {
	later := fixedNow.Add(72 * time.Hour)
	cases := []struct {
		name    string
		mutate  func(s *Store) error
		changes []string // the only fields it may change
	}{
		{"SetIdentity", func(s *Store) error {
			return s.SetIdentity("work", Identity{Email: "new@example.com", Org: "New Org", Plan: "pro", SignedInAt: later})
		}, []string{"Email", "Org", "Plan", "SignedInAt"}},
		{"ClearSignIn", func(s *Store) error { return s.ClearSignIn("work") }, []string{"Org", "Plan", "SignedInAt"}},
		{"Hide", func(s *Store) error { return s.Hide("work") }, []string{"Hidden", "Email", "Org", "Plan", "SignedInAt"}},
		{"Unhide", func(s *Store) error { return s.Unhide("work") }, []string{"Hidden"}},
		{"Relabel", func(s *Store) error { return s.Relabel("work", "office") }, []string{"Label"}},
		{"Touch", func(s *Store) error { return s.Touch("work") }, []string{"LastUsedAt"}},
		{"SetBudget", func(s *Store) error { return s.SetBudget("work", 7, 9) }, []string{"DailyBudget", "WeeklyBudget"}},
		{"Create", func(s *Store) error { _, err := s.Create("fresh", noFolders); return err }, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newMetaStore(t)
			addLegacy(t, s, "work", fullMeta(t))
			addLegacy(t, s, "other", fullMeta(t))
			st, err := s.load()
			if err != nil {
				t.Fatal(err)
			}
			st.Active = "other"
			if err := s.save(st); err != nil {
				t.Fatal(err)
			}
			before := stored(t, s, "work").Meta
			otherBefore := stored(t, s, "other").Meta

			if err := c.mutate(s); err != nil {
				t.Fatal(err)
			}

			allowed := map[string]bool{}
			for _, name := range c.changes {
				allowed[name] = true
			}
			after := stored(t, s, "work").Meta
			bv, av := reflect.ValueOf(before), reflect.ValueOf(after)
			for i := 0; i < bv.NumField(); i++ {
				name := bv.Type().Field(i).Name
				if !allowed[name] && !reflect.DeepEqual(bv.Field(i).Interface(), av.Field(i).Interface()) {
					t.Errorf("%s changed Meta.%s from %v to %v", c.name, name, bv.Field(i).Interface(), av.Field(i).Interface())
				}
			}
			if got := stored(t, s, "other").Meta; got != otherBefore {
				t.Errorf("%s changed another account: %+v then %+v", c.name, otherBefore, got)
			}
			if st, _ := s.load(); st.Active != "other" {
				t.Errorf("%s moved the active pointer to %q", c.name, st.Active)
			}
		})
	}
}
