package copilot

import (
	"bufio"
	"errors"
	"io"
	"os"
	"time"

	"github.com/dhitalkamal/clalt/internal/feed"
)

const defaultFirstReadBytes = 64 << 20

// entry is what a poll remembers of one events file: how far it has folded
// (always the end of a complete line), the file's size and mtime then, and the
// summary so far.
type entry struct {
	offset int64
	size   int64
	mtime  time.Time
	sum    summary
}

// read returns the summary of the events file, reading only what is new since
// the last call. the caller holds r.mu.
func (r *Reader) read(id, path string, tail int64) (summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return summary{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return summary{}, err
	}
	if r.cache == nil {
		r.cache = map[string]*entry{}
	}
	e := r.cache[id]
	if e != nil && e.size == fi.Size() && e.mtime.Equal(fi.ModTime()) {
		return e.sum.view(), nil
	}
	// a smaller file, or an older one, was replaced: fold it again from the top
	if e == nil || fi.Size() < e.offset || fi.ModTime().Before(e.mtime) {
		e = &entry{sum: summary{State: feed.StateIdle, base: feed.StateIdle}}
		first := r.FirstReadBytes
		if first <= 0 {
			first = defaultFirstReadBytes
		}
		if fi.Size() > first {
			e.offset = fi.Size() - tail
			if e.offset < 0 {
				e.offset = 0
			}
			if e.offset > 0 {
				if e.offset, err = skipPartial(f, e.offset); err != nil {
					return summary{}, err
				}
			}
		}
	}
	if _, err := f.Seek(e.offset, io.SeekStart); err != nil {
		return summary{}, err
	}
	br := bufio.NewReader(io.LimitReader(f, fi.Size()-e.offset))
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			// no newline: a partial line, left for the next read
			if !errors.Is(err, io.EOF) {
				return summary{}, err
			}
			break
		}
		e.offset += int64(len(line))
		e.sum.fold(line)
	}
	e.size, e.mtime = fi.Size(), fi.ModTime()
	r.cache[id] = e
	return e.sum.view(), nil
}

// skipPartial moves off to just after the next newline at or past it, so a
// read that starts mid file begins on a whole line.
func skipPartial(f *os.File, off int64) (int64, error) {
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadSlice('\n')
		off += int64(len(line))
		if err == nil || errors.Is(err, io.EOF) {
			return off, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return 0, err
		}
	}
}
