package dashboard_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dhitalkamal/clalt/internal/dashboard"
)

var randomKeys = []string{
	"j", "k", "up", "down", "left", "right", "h", "l", "tab", "shift+tab", "[", "]", "1", "2", "3", "4", "5",
	"/", "a", "s", "enter", "esc", "?", "e", "x", "y", "n", "backspace", "space", "0", "9", ".", "z", "q", "r", "d",
}

var randomSizes = [][2]int{{60, 16}, {64, 20}, {71, 18}, {72, 20}, {80, 24}, {95, 30}, {96, 30}, {100, 30}, {140, 40}, {59, 15}, {50, 14}}

// any sequence of keys and resizes must leave a frame that fits the terminal
func TestRandomKeySequencesKeepEveryFrameValid(t *testing.T) {
	rng := rand.New(rand.NewSource(20251015))
	for seq := 0; seq < 60; seq++ {
		ascii := seq%2 == 0
		sz := randomSizes[rng.Intn(len(randomSizes))]
		m := sized(config{ascii: ascii}, sz[0], sz[1])
		var trail []string
		for step := 0; step < 30; step++ {
			if rng.Intn(10) == 0 {
				sz = randomSizes[rng.Intn(len(randomSizes))]
				m = dashboard.Resize(m, sz[0], sz[1])
				trail = append(trail, fmt.Sprintf("resize %dx%d", sz[0], sz[1]))
			} else {
				key := randomKeys[rng.Intn(len(randomKeys))]
				if ascii && key == "a" {
					key = "z" // a toggles ascii mode, which these runs check stays on
				}
				var cmd tea.Cmd
				m, cmd = press(m, key)
				trail = append(trail, key)
				// let quick commands finish so flashes and reloads show up
				if msg := dashboard.RunCmd(cmd, 5*time.Millisecond); msg != nil {
					m, _ = m.Update(msg)
				}
			}
			out := screen(m)
			label := fmt.Sprintf("seq %d after %s (%dx%d ascii=%v)", seq, strings.Join(trail, " "), sz[0], sz[1], ascii)
			checkFrame(t, label, out, sz[0], sz[1])
			if ascii && !dashboard.IsASCII(out) {
				t.Fatalf("%s: non-ascii output in ascii mode", label)
			}
			if t.Failed() {
				return
			}
		}
	}
}
