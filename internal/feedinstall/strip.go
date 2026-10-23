package feedinstall

import "encoding/json"

// strip takes clalt's handlers and statusLine out of raw, and puts original back
// as the statusLine when there is one. it reports how many handlers went.
func strip(raw []byte, original json.RawMessage) (out []byte, removed int, err error) {
	o, err := parseObject(raw)
	if err != nil {
		return nil, 0, err
	}
	hooks, err := hooksOf(o)
	if err != nil {
		return nil, 0, err
	}
	for ev := range hooks {
		groups, err := groupsOf(hooks, ev)
		if err != nil {
			return nil, 0, err
		}
		var kept []json.RawMessage
		before := removed
		for _, g := range groups {
			ng, n, keep := stripGroup(g)
			removed += n
			if keep {
				kept = append(kept, ng)
			}
		}
		if removed == before {
			continue
		}
		if len(kept) == 0 {
			delete(hooks, ev)
		} else {
			hooks[ev], _ = marshal(kept)
		}
	}
	if removed > 0 {
		if len(hooks) == 0 {
			delete(o, "hooks")
		} else {
			o["hooks"], _ = marshal(hooks)
		}
	}
	if line, ok := o["statusLine"]; ok && lineIsOurs(line) {
		removed++
		if original != nil {
			o["statusLine"] = original
		} else {
			delete(o, "statusLine")
		}
	}
	if removed == 0 {
		return raw, 0, nil
	}
	out, err = encode(o)
	return out, removed, err
}

func lineIsOurs(line json.RawMessage) bool {
	var l handler
	return json.Unmarshal(line, &l) == nil && hasMarker(l.Command)
}

// stripGroup drops clalt's handlers from one matcher group. a group that holds
// none is returned as it is, byte for byte. keep is false for a group that has
// nothing left.
func stripGroup(g json.RawMessage) (out json.RawMessage, removed int, keep bool) {
	if !groupHasMarker(g) {
		return g, 0, true
	}
	var o object
	var hs []json.RawMessage
	if json.Unmarshal(g, &o) != nil || json.Unmarshal(o["hooks"], &hs) != nil {
		return g, 0, true
	}
	var left []json.RawMessage
	for _, h := range hs {
		var hd handler
		if json.Unmarshal(h, &hd) == nil && hasMarker(hd.Command) {
			removed++
			continue
		}
		left = append(left, h)
	}
	if len(left) == 0 {
		return nil, removed, false
	}
	o["hooks"], _ = marshal(left)
	out, _ = marshal(o)
	return out, removed, true
}
