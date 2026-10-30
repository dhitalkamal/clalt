package reviews

import (
	"context"
	"strings"
)

// chunkResult is what a set of claude runs on one pull request came to.
type chunkResult struct {
	Findings   []Finding
	Summary    string
	Transcript string
	// Error is set when the review is partial or nothing was reviewed.
	Error string
}

// pick is the account for the next claude run, errNoRoom when none has room.
func (s *Service) pick() (Candidate, error) {
	cand, ok := Pick(s.Accounts())
	if !ok {
		return Candidate{}, errNoRoom
	}
	return cand, nil
}

// runOnce is one claude run in wt with prompt and session id sid. it returns
// the parsed reply and the session id claude reports (sid when it reports
// none). each run has its own reviewTimeout.
func (s *Service) runOnce(ctx context.Context, wt string, cand Candidate, prompt, sid string) ([]Finding, string, string, error) {
	rctx, cancel := context.WithTimeout(ctx, reviewTimeout)
	defer cancel()
	out, err := s.Claude.Run(rctx, wt, s.EnvFor(cand.ConfigDir), prompt, s.ClaudeBin, claudeArgs(sid)...)
	if err != nil {
		return nil, "", sid, err
	}
	text, got := ParseEnvelope(string(out))
	if got != "" {
		sid = got
	}
	findings, summary, _ := ParseFindings(text)
	return findings, summary, sid, nil
}

// reviewChunks runs claude once per chunk in order and merges the replies.
// cand is the account of the first run, every later run picks its own with
// pick, and errNoRoom from a pick stops the review (returned, never recorded
// as a chunk failure). first is the session id of the first run. one chunk gets exactly the usual
// prompt of the whole diff. 2+ chunks end with one cross-file pass. a failed
// chunk run is recorded in Error and the rest still run; the cross-file pass
// runs only when every chunk worked. a cancelled context is returned as an
// error.
func (s *Service) reviewChunks(ctx context.Context, wt string, pr PR, info PRInfo, cand Candidate, chunks []chunk, diff, first string) (chunkResult, error) {
	if len(chunks) == 1 {
		// keep the prompt byte for byte what a single run always got
		f, sum, sid, err := s.runOnce(ctx, wt, cand, buildPrompt(pr, info, diff), first)
		if err != nil {
			return chunkResult{}, err
		}
		return chunkResult{Findings: f, Summary: sum, Transcript: sid}, nil
	}
	var parts []part
	var fails []chunkFailure
	transcript := first
	for i, c := range chunks {
		id := first
		if i > 0 {
			id = newSessionID()
		}
		rc := cand
		if i > 0 {
			var perr error
			if rc, perr = s.pick(); perr != nil {
				return chunkResult{}, perr
			}
		}
		f, sum, sid, err := s.runOnce(ctx, wt, rc, buildChunkPrompt(pr, info, c, i+1, len(chunks)), id)
		if err != nil {
			if ctx.Err() != nil {
				// a cancelled run must never turn into a saved partial review
				return chunkResult{}, err
			}
			fails = append(fails, chunkFailure{No: i + 1, Of: len(chunks), Files: c.Files, Err: err})
			continue
		}
		if len(parts) == 0 {
			transcript = sid
		}
		parts = append(parts, part{Findings: f, Summary: sum})
	}
	if len(parts) == 0 {
		return chunkResult{Findings: []Finding{}, Transcript: first, Error: failureText(fails)}, nil
	}
	if len(fails) > 0 {
		f, sum := mergeParts(parts)
		return chunkResult{Findings: f, Summary: sum, Transcript: transcript, Error: partialPrefix + failureText(fails)}, nil
	}
	cc, err := s.pick()
	if err != nil {
		return chunkResult{}, err
	}
	cf, csum, _, err := s.runOnce(ctx, wt, cc, buildCrossPrompt(pr, info, chunkFiles(chunks), chunkSummaries(parts)), newSessionID())
	if err != nil {
		if ctx.Err() != nil {
			return chunkResult{}, err
		}
		f, sum := mergeParts(parts)
		reason := tidy(err.Error(), maxReason)
		return chunkResult{Findings: f, Summary: sum, Transcript: transcript, Error: partialPrefix + "cross-file pass failed: " + reason}, nil
	}
	parts = append(parts, part{Findings: cf, Summary: csum})
	f, sum := mergeParts(parts)
	return chunkResult{Findings: f, Summary: sum, Transcript: transcript}, nil
}

// chunkFiles is the file names of all chunks in order.
func chunkFiles(chunks []chunk) []string {
	var files []string
	for _, c := range chunks {
		files = append(files, c.Files...)
	}
	return files
}

// chunkSummaries is the non-empty summaries of parts in order.
func chunkSummaries(parts []part) []string {
	var sums []string
	for _, p := range parts {
		if strings.TrimSpace(p.Summary) != "" {
			sums = append(sums, p.Summary)
		}
	}
	return sums
}
