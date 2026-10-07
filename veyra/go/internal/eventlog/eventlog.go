// Package eventlog persists a run as newline-delimited protojson events.
//
// JSONL (not SQLite) is deliberate for v0.1: the log is append-only, trivially
// replayable, diffable and requires no native dependency in the kernel.
package eventlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

var marshaler = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: false}

// Writer appends events for one run.
type Writer struct {
	mu   sync.Mutex
	f    *os.File
	w    *bufio.Writer
	path string
}

// NewWriter creates (or appends to) the event log at path.
func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{f: f, w: bufio.NewWriter(f), path: path}, nil
}

func (w *Writer) Path() string { return w.path }

// Append serialises one event as a single line.
func (w *Writer) Append(ev *veyrav1.RunEvent) error {
	b, err := marshaler.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.w.Write(b); err != nil {
		return err
	}
	if err := w.w.WriteByte('\n'); err != nil {
		return err
	}
	return w.w.Flush()
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.w.Flush(); err != nil {
		_ = w.f.Close()
		return err
	}
	return w.f.Close()
}

// Read loads a full run log from disk.
func Read(path string) ([]*veyrav1.RunEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []*veyrav1.RunEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		ev := &veyrav1.RunEvent{}
		if err := protojson.Unmarshal(line, ev); err != nil {
			return nil, fmt.Errorf("line %d: %w", len(events)+1, err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// WriteCheckpoint stores a portable execution state snapshot.
func WriteCheckpoint(dir string, cpid string, state *veyrav1.CommonExecutionState) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, cpid+".json")
	b, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true, Indent: "  "}.Marshal(state)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ReadCheckpoint loads a checkpoint written by WriteCheckpoint.
func ReadCheckpoint(path string) (*veyrav1.CommonExecutionState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &veyrav1.CommonExecutionState{}
	if err := protojson.Unmarshal(b, state); err != nil {
		return nil, err
	}
	return state, nil
}

// MarshalJSON is a small helper for embedding arbitrary payloads in events.
func MarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}