// Package mcp is clalt's read-only MCP server: JSON-RPC 2.0 over stdio, one
// message per line, with four tools that read the same sources as the office.
// it writes nothing to disk and opens no port. stdout carries protocol lines
// only; anything else goes through Backend.Logf to stderr.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/dhitalkamal/clalt/internal/office"
	"github.com/dhitalkamal/clalt/internal/office/summary"
	"github.com/dhitalkamal/clalt/internal/version"
)

// protocol versions the server answers to. the first is the latest handshake
// version, modernVersion is the per-request metadata revision.
const (
	latestVersion = "2025-11-25"
	modernVersion = "2026-07-28"
	metaVersion   = "io.modelcontextprotocol/protocolVersion"
	maxLine       = 1 << 20
)

var legacyVersions = []string{latestVersion, "2025-06-18", "2025-03-26", "2024-11-05"}

// json-rpc error codes
const (
	codeParse       = -32700
	codeInvalid     = -32600
	codeNoMethod    = -32601
	codeBadParams   = -32602
	codeUnsupported = -32022
)

// Backend is everything the tools read. Snapshot is office.Build over the real
// sources, Day is the office summary service. Now and Logf may be nil.
type Backend struct {
	Snapshot func() (office.Snapshot, error)
	Day      func(day string) (summary.Summary, error)
	Now      func() time.Time
	Logf     func(format string, args ...any)
}

func (b Backend) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b Backend) logf(format string, args ...any) {
	if b.Logf != nil {
		b.Logf(format, args...)
	}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve reads requests from in until it ends or ctx is done, and answers on
// out. a bad line is answered with an error and the loop goes on.
func Serve(ctx context.Context, in io.Reader, out io.Writer, b Backend) error {
	rd := bufio.NewReaderSize(in, 64*1024)
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	for ctx.Err() == nil {
		line, tooBig, err := readLine(rd)
		if len(bytes.TrimSpace(line)) > 0 || tooBig {
			var resp *response
			if tooBig {
				resp = fail(json.RawMessage("null"), codeInvalid, "message too large", nil)
			} else {
				resp = handle(line, b)
			}
			if resp != nil {
				// Encode ends the value with one newline and escapes inner ones
				if werr := enc.Encode(resp); werr != nil {
					return werr
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
	return ctx.Err()
}

// readLine reads one line up to maxLine bytes. a longer one is skipped to its
// end and reported as tooBig.
func readLine(rd *bufio.Reader) (line []byte, tooBig bool, err error) {
	for {
		part, isPrefix, e := rd.ReadLine()
		if e != nil {
			return line, tooBig, e
		}
		if !tooBig {
			if len(line)+len(part) > maxLine {
				tooBig, line = true, nil
			} else {
				line = append(line, part...)
			}
		}
		if !isPrefix {
			return line, tooBig, nil
		}
	}
}

func fail(id json.RawMessage, code int, msg string, data any) *response {
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg, Data: data}}
}

// handle answers one line, nil for a notification.
func handle(line []byte, b Backend) *response {
	null := json.RawMessage("null")
	trimmed := bytes.TrimSpace(line)
	if !json.Valid(trimmed) {
		return fail(null, codeParse, "parse error", nil)
	}
	if trimmed[0] != '{' {
		return fail(null, codeInvalid, "a request must be one json object", nil)
	}
	var req request
	if err := json.Unmarshal(trimmed, &req); err != nil {
		return fail(null, codeInvalid, "invalid request", nil)
	}
	id := req.ID
	if len(id) == 0 {
		id = null
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return fail(id, codeInvalid, "invalid request", nil)
	}
	if len(req.ID) == 0 {
		return nil // a notification is never answered, known or not
	}
	modern, bad := metaOf(req.Params)
	if modern && !slices.Contains(append([]string{modernVersion}, legacyVersions...), bad) {
		return fail(id, codeUnsupported, "Unsupported protocol version", map[string]any{
			"supported": append([]string{modernVersion}, legacyVersions...), "requested": bad})
	}
	result, rerr := dispatch(req, b)
	if rerr != nil {
		return &response{JSONRPC: "2.0", ID: id, Error: rerr}
	}
	if modern {
		result["resultType"] = "complete"
	}
	return &response{JSONRPC: "2.0", ID: id, Result: result}
}

// metaOf reports whether params carry the per-request protocol version of the
// modern revision, and which one.
func metaOf(params json.RawMessage) (bool, string) {
	var p struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	if json.Unmarshal(params, &p) != nil {
		return false, ""
	}
	raw, ok := p.Meta[metaVersion]
	if !ok {
		return false, ""
	}
	var v string
	_ = json.Unmarshal(raw, &v)
	return true, v
}

func dispatch(req request, b Backend) (map[string]any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := latestVersion
		if slices.Contains(legacyVersions, p.ProtocolVersion) {
			v = p.ProtocolVersion
		}
		out := serverInfo()
		out["protocolVersion"] = v
		return out, nil
	case "server/discover":
		out := serverInfo()
		out["supportedVersions"] = append([]string{modernVersion}, legacyVersions...)
		out["_meta"] = map[string]any{"io.modelcontextprotocol/serverInfo": out["serverInfo"]}
		return out, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		return callTool(req.Params, b)
	}
	return nil, &rpcError{Code: codeNoMethod, Message: "method not found: " + req.Method}
}

func serverInfo() map[string]any {
	return map[string]any{
		"capabilities": map[string]any{"tools": map[string]any{}},
		"serverInfo":   map[string]any{"name": "clalt", "title": "clalt accounts", "version": version.Info().Version},
		"instructions": "read-only view of the clalt accounts on this machine: limits, the best account to use now, running sessions and the day summary",
	}
}
