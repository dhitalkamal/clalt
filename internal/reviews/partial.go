package reviews

import (
	"fmt"
	"strings"
)

// maxReason caps each failure reason in a partial review's error text.
const maxReason = 300

// partialPrefix starts the error of a review that kept some findings.
const partialPrefix = "partial review: "

// chunkFailure is one chunk run that failed.
type chunkFailure struct {
	No, Of int
	Files  []string
	Err    error
}

// failureText joins the failures in chunk order. each reason is cut on its
// own so the chunk names always survive.
func failureText(fails []chunkFailure) string {
	var out []string
	for _, f := range fails {
		out = append(out, fmt.Sprintf("chunk %d of %d failed (files: %s): %s",
			f.No, f.Of, strings.Join(f.Files, ", "), errText(f.Err.Error(), maxReason)))
	}
	return strings.Join(out, "; ")
}
