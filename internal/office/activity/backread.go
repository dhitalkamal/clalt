package activity

import (
	"bytes"
	"io"
)

// chunk is how much is read per step going backwards.
const chunk = 256 * 1024

// backReader hands out the lines of a file from the end towards the start,
// reading no more than max bytes in all. a line longer than maxLine is
// skipped and counted, not returned.
type backReader struct {
	f          io.ReaderAt
	max        int64
	maxLine    int
	scanned    int64
	carry      []byte // bytes [carryStart, carryStart+len) holding the line being closed
	carryStart int64
	oversized  bool // the line being closed is past maxLine, its bytes are dropped
	skipped    int
	// exhausted is set when the read budget ran out before the start of the file
	exhausted bool
}

func newBackReader(f io.ReaderAt, end, max int64, maxLine int) *backReader {
	return &backReader{f: f, max: max, maxLine: maxLine, carryStart: end}
}

// next returns the next older line and the offset it starts at. ok is false at
// the start of the file or when the budget ran out (see exhausted). the line is
// without its newline and is only good until the next call.
func (b *backReader) next() (line []byte, start int64, ok bool) {
	for {
		if !b.oversized {
			if i := bytes.LastIndexByte(b.carry, '\n'); i >= 0 {
				line, start = b.carry[i+1:], b.carryStart+int64(i)+1
				b.carry = b.carry[:i]
				if len(line) > b.maxLine {
					b.skipped++
					continue
				}
				return line, start, true
			}
			if len(b.carry) > b.maxLine {
				b.oversized, b.carry = true, nil
			}
		}
		if b.carryStart == 0 {
			if b.oversized {
				b.oversized = false
				b.skipped++
				return nil, 0, false
			}
			if len(b.carry) > 0 {
				line, b.carry = b.carry, nil
				if len(line) > b.maxLine {
					b.skipped++
					return nil, 0, false
				}
				return line, 0, true
			}
			return nil, 0, false
		}
		n := int64(chunk)
		if n > b.carryStart {
			n = b.carryStart
		}
		if left := b.max - b.scanned; n > left {
			n = left
		}
		if n <= 0 {
			b.exhausted = true
			return nil, 0, false
		}
		buf := make([]byte, n)
		if _, err := b.f.ReadAt(buf, b.carryStart-n); err != nil && err != io.EOF {
			b.exhausted = true
			return nil, 0, false
		}
		b.scanned += n
		b.carryStart -= n
		if b.oversized {
			// the rest of the long line is in the chunk's tail: keep what is before its last newline
			i := bytes.LastIndexByte(buf, '\n')
			if i < 0 {
				continue
			}
			b.oversized = false
			b.skipped++
			b.carry = buf[:i]
			continue
		}
		b.carry = append(buf, b.carry...)
	}
}
