package dashboard

type column struct {
	head  string
	right bool
}

func table(cols []column, rows [][]rich, gapW int) (head rich, body []rich) {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c.head)
		for _, row := range rows {
			widths[i] = max(widths[i], row[i].width())
		}
	}
	build := func(cells []rich) rich {
		var line rich
		for i, c := range cols {
			if i > 0 {
				line = append(line, gap(gapW))
			}
			cell := cells[i]
			pad := gap(widths[i] - cell.width())
			if c.right {
				line = append(line, pad)
				line = append(line, cell...)
			} else {
				line = append(line, cell...)
				line = append(line, pad)
			}
		}
		return line
	}
	heads := make([]rich, len(cols))
	for i, c := range cols {
		heads[i] = rich{dim(c.head)}
	}
	head = build(heads)
	for _, row := range rows {
		body = append(body, build(row))
	}
	return head, body
}

func tableWidth(cols []column, rows [][]rich, gapW int) int {
	head, _ := table(cols, rows, gapW)
	return head.width()
}

// thumbRows marks the viewport rows that show the scroll thumb, nil when
// everything fits
func thumbRows(total, vh, off int) []bool {
	if total <= vh || vh <= 0 {
		return nil
	}
	size := max(1, vh*vh/total)
	maxOff := total - vh
	pos := 0
	if maxOff > 0 {
		pos = off * (vh - size) / maxOff
	}
	out := make([]bool, vh)
	for i := pos; i < pos+size && i < vh; i++ {
		out[i] = true
	}
	return out
}
