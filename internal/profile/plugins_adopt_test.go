package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// adoptFixture is a shared home with a split plugins folder and an account
// under root/accounts/<id>, so the backups land in root/backups/plugins/<id>.
func adoptFixture(t *testing.T) (root, shared, acct string) {
	t.Helper()
	root, shared = sharedFixture(t)
	splitShared(t, shared)
	acct = filepath.Join(root, "accounts", "work")
	mkdirT(t, filepath.Join(acct, "plugins"))
	return root, shared, acct
}

func setAge(t *testing.T, path string, age time.Duration) {
	t.Helper()
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func backupsOf(t *testing.T, root string) []string {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(root, "backups", "plugins", "work"))
	var out []string
	for _, e := range entries {
		out = append(out, filepath.Join(root, "backups", "plugins", "work", e.Name()))
	}
	return out
}

func TestPerAccountPluginsEntriesAreNeverLinkedOrWarned(t *testing.T) {
	root, shared, acct := adoptFixture(t)
	for _, n := range []string{"plugin-directory-cache-v2.json", ".last_inuse_sweep"} {
		writeT(t, filepath.Join(shared, "plugins", n), "shared")
		writeT(t, filepath.Join(acct, "plugins", n), "mine")
	}

	rep := mustSync(t, acct, shared)

	for _, n := range []string{"plugin-directory-cache-v2.json", ".last_inuse_sweep"} {
		p := filepath.Join(acct, "plugins", n)
		if fi, _ := os.Lstat(p); fi == nil || fi.Mode()&os.ModeSymlink != 0 || readT(t, p) != "mine" {
			t.Errorf("%s was touched", n)
		}
	}
	if !rep.Complete() || len(backupsOf(t, root)) != 0 {
		t.Errorf("report = %+v, backups %v", rep, backupsOf(t, root))
	}
	steps, err := Plan(acct, shared)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range steps {
		if strings.Contains(st.Name, "plugin-directory-cache") || strings.Contains(st.Name, "last_inuse") {
			t.Errorf("plan mentions %+v", st)
		}
	}
}

func TestSyncAdoptsANewerAccountCopy(t *testing.T) {
	root, shared, acct := adoptFixture(t)
	sharedFile := filepath.Join(shared, "plugins", "known_marketplaces.json")
	accFile := filepath.Join(acct, "plugins", "known_marketplaces.json")
	writeT(t, sharedFile, "old shared")
	writeT(t, accFile, "new account")
	setAge(t, sharedFile, time.Hour)

	rep := mustSync(t, acct, shared)

	if got := readT(t, sharedFile); got != "new account" {
		t.Errorf("shared = %q", got)
	}
	if !linksTo(accFile, sharedFile) {
		t.Error("the account entry is not a link")
	}
	b := backupsOf(t, root)
	if len(b) != 1 || !strings.HasPrefix(filepath.Base(b[0]), "known_marketplaces.json.") || readT(t, b[0]) != "old shared" {
		t.Fatalf("backups = %v", b)
	}
	if len(rep.Adopted) != 1 || rep.Adopted[0].Name != "plugins/known_marketplaces.json" || rep.Adopted[0].Backup != b[0] {
		t.Errorf("Adopted = %+v", rep.Adopted)
	}
	if !rep.Complete() {
		t.Errorf("report = %+v", rep)
	}
	// second run has nothing to do
	again := mustSync(t, acct, shared)
	if len(again.Adopted) != 0 || len(again.Linked) != 0 || len(backupsOf(t, root)) != 1 {
		t.Errorf("second run = %+v", again)
	}
}

func TestSyncKeepsANewerSharedCopyAndBacksUpTheAccountOne(t *testing.T) {
	root, shared, acct := adoptFixture(t)
	sharedFile := filepath.Join(shared, "plugins", "installed_plugins.json")
	accFile := filepath.Join(acct, "plugins", "installed_plugins.json")
	writeT(t, sharedFile, "new shared")
	writeT(t, accFile, "old account")
	setAge(t, accFile, time.Hour)

	rep := mustSync(t, acct, shared)

	if readT(t, sharedFile) != "new shared" || !linksTo(accFile, sharedFile) {
		t.Error("shared was changed or the account is not linked")
	}
	b := backupsOf(t, root)
	if len(b) != 1 || readT(t, b[0]) != "old account" || len(rep.Adopted) != 1 {
		t.Errorf("backups %v, report %+v", b, rep)
	}
}

func TestSyncAdoptsWhenTheSharedCopyIsMissing(t *testing.T) {
	root, shared, acct := adoptFixture(t)
	accFile := filepath.Join(acct, "plugins", "plugin-catalog-cache.json")
	writeT(t, accFile, "only copy")

	rep := mustSync(t, acct, shared)

	sharedFile := filepath.Join(shared, "plugins", "plugin-catalog-cache.json")
	if readT(t, sharedFile) != "only copy" || !linksTo(accFile, sharedFile) {
		t.Error("not adopted")
	}
	if len(backupsOf(t, root)) != 0 || len(rep.Adopted) != 1 || rep.Adopted[0].Backup != "" {
		t.Errorf("backups %v, report %+v", backupsOf(t, root), rep)
	}
}

func TestSyncDoesNotAdoptWhatIsNotARegularSmallFile(t *testing.T) {
	root, shared, acct := adoptFixture(t)
	dirIn := filepath.Join(acct, "plugins", "installed_plugins.json")
	mkdirT(t, filepath.Join(dirIn, "x"))
	linkIn := filepath.Join(acct, "plugins", "known_marketplaces.json")
	mustSymlink(t, filepath.Join(root, "elsewhere"), linkIn)
	bigIn := filepath.Join(acct, "plugins", "plugin-catalog-cache.json")
	f, err := os.Create(bigIn)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxAdoptSize + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	writeT(t, filepath.Join(shared, "plugins", "plugin-catalog-cache.json"), "shared")
	before := readT(t, filepath.Join(shared, "plugins", "installed_plugins.json"))

	rep := mustSync(t, acct, shared)

	if len(rep.Adopted) != 0 || len(rep.Kept) < 2 || len(backupsOf(t, root)) != 0 {
		t.Errorf("report = %+v", rep)
	}
	if !isRealDir(t, dirIn) || readT(t, filepath.Join(shared, "plugins", "installed_plugins.json")) != before {
		t.Error("the directory or the shared file changed")
	}
	if readT(t, filepath.Join(shared, "plugins", "plugin-catalog-cache.json")) != "shared" {
		t.Error("shared changed for an oversized file")
	}
}

func TestPlanShowsAnAdoptWithoutChangingAnything(t *testing.T) {
	root, shared, acct := adoptFixture(t)
	accFile := filepath.Join(acct, "plugins", "known_marketplaces.json")
	writeT(t, accFile, "new account")
	before := listTree(t, shared)

	steps, err := Plan(acct, shared)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, st := range steps {
		if st.Name == "plugins/known_marketplaces.json" {
			found = st.Action == Adopt && st.Source == filepath.Join(shared, "plugins", "known_marketplaces.json")
		}
	}
	if !found {
		t.Errorf("steps = %+v", steps)
	}
	if readT(t, accFile) != "new account" || len(backupsOf(t, root)) != 0 {
		t.Error("plan changed something")
	}
	if len(listTree(t, shared)) != len(before) {
		t.Error("the shared home changed")
	}
}
