package migrate

import (
	"crypto/sha256"
	"io"
	"os"
)

// compareFiles says whether other holds the same bytes as src. when it does
// not, it says whether other is merely the start of src, which is what an
// unfinished copy or a transcript that kept growing looks like.
func compareFiles(src, other io.Reader, srcSize, otherSize int64) (result, Reason, error) {
	if otherSize > srcSize {
		return differs, ReasonDiffers, nil
	}
	a, err := digest(io.LimitReader(src, otherSize))
	if err != nil {
		return 0, "", err
	}
	b, err := digest(other)
	if err != nil {
		return 0, "", err
	}
	switch {
	case a != b:
		return differs, ReasonDiffers, nil
	case otherSize == srcSize:
		return identical, "", nil
	}
	return differs, ReasonShorter, nil
}

func digest(r io.Reader) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return sum, err
	}
	h.Sum(sum[:0])
	return sum, nil
}

func compareOpen(src, other *os.File) (result, Reason, error) {
	si, err := src.Stat()
	if err != nil {
		return 0, "", err
	}
	oi, err := other.Stat()
	if err != nil {
		return 0, "", err
	}
	return compareFiles(src, other, si.Size(), oi.Size())
}

func compareOnDisk(src, other string) (result, Reason, error) {
	a, err := os.Open(src)
	if err != nil {
		return 0, "", err
	}
	defer a.Close()
	b, err := os.Open(other)
	if err != nil {
		return 0, "", err
	}
	defer b.Close()
	return compareOpen(a, b)
}
