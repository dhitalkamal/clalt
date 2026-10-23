package migrate

// plan stands in for placing a file in a dry run. nothing is written, so a file
// an earlier profile would have placed has to be remembered: a later profile
// holding the same path is then judged against it, as a real run would find it
// on disk.
func (c *copier) plan(src, rel string) (placement, error) {
	// open it as the real copy would, a plain stat says yes to a file that
	// cannot be read
	f, err := c.openSrc(src)
	if err != nil {
		return placement{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return placement{}, err
	}
	if prev, ok := c.planned[rel]; ok {
		res, reason, err := compareOnDisk(src, prev)
		return placement{res: res, reason: reason}, err
	}
	c.planned[rel] = src
	return placement{res: placed, bytes: st.Size()}, nil
}
