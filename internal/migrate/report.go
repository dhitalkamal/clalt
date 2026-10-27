package migrate

import "os"

// Status says what a profile's projects folder turned out to be: nothing there,
// the shared folder itself, a link that was not followed, or a real folder that
// was copied.
type Status string

const (
	StatusNothing Status = "nothing"
	StatusShared  Status = "shared"
	StatusLink    Status = "link"
	StatusFolder  Status = "folder"
)

// Reason says why a file was not copied over what is already there: the file
// there has other content, something that is not a file is in the way, or the
// file there is only the start of the source.
type Reason string

const (
	ReasonDiffers  Reason = "differs"
	ReasonInTheWay Reason = "inTheWay"
	ReasonShorter  Reason = "shorter"
)

// Conflict is a file left alone because something else is at its path. Path is
// relative to the profile's projects folder.
type Conflict struct {
	Path   string `json:"path"`
	Reason Reason `json:"reason"`
}

// Skip is an entry inside a projects folder that was left alone because it is
// not a regular file. Target is where a link points, as written.
type Skip struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Target string `json:"target,omitempty"`
}

// Failure is a file or folder that could not be copied. Path is relative to the
// profile's projects folder, and Kind says whether it names a file or a folder.
type Failure struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Error string `json:"error"`
}

// KindFile and KindFolder say what a Failure's Path names.
const (
	KindFile   = "file"
	KindFolder = "folder"
)

// LinkOfProfile and LinkOfProjects say which entry of a StatusLink profile is
// the link.
const (
	LinkOfProfile  = "profile"
	LinkOfProjects = "projects"
)

// Profile is the outcome for one profile folder. the counters only mean
// something for StatusFolder. for StatusLink, LinkOf names what is a link (the
// profile folder or its projects folder), Target is where it leads and Error is
// why it could not be resolved, if it could not.
type Profile struct {
	Name           string     `json:"name"`
	Status         Status     `json:"status"`
	LinkOf         string     `json:"linkOf,omitempty"`
	Target         string     `json:"target,omitempty"`
	Error          string     `json:"error,omitempty"`
	Copied         int        `json:"copied"`
	Bytes          int64      `json:"bytes"`
	InPlace        int        `json:"writtenInPlace"`
	AlreadyThere   int        `json:"alreadyThere"`
	Conflicts      []Conflict `json:"conflicts"`
	Skipped        int        `json:"skipped"`
	SkippedEntries []Skip     `json:"skippedEntries"`
	Failed         []Failure  `json:"failed"`
}

// Totals adds up every profile. Linked counts the profiles whose projects folder
// or whose own folder is a link that was not followed.
type Totals struct {
	Copied       int   `json:"copied"`
	Bytes        int64 `json:"bytes"`
	InPlace      int   `json:"writtenInPlace"`
	AlreadyThere int   `json:"alreadyThere"`
	Conflicts    int   `json:"conflicts"`
	Skipped      int   `json:"skipped"`
	Failed       int   `json:"failed"`
	Linked       int   `json:"linked"`
}

// Report is what a run did, or what a dry run would do.
type Report struct {
	Source   string    `json:"source"`
	Dest     string    `json:"dest"`
	DryRun   bool      `json:"dryRun"`
	Profiles []Profile `json:"profiles"`
	Total    Totals    `json:"total"`
}

// Options shapes a run. DryRun reads both sides and reports what a real run
// would do, without creating or changing anything.
type Options struct {
	DryRun bool

	// seams for tests, nil keeps the real thing: wrap swaps the destination for
	// another store, openSrc opens a source file.
	wrap    func(store) store
	openSrc func(path string) (*os.File, error)
}
