// Package tasklog captures per-task log lines so the Web UI can show what a
// single generation run did, while still forwarding every line to the
// process logger.
package tasklog

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// MaxLines caps the lines kept per task so a huge library cannot bloat SQLite.
const MaxLines = 5000

type ctxKey struct{}

// Recorder collects log lines of one task.
type Recorder struct {
	mu      sync.Mutex
	lines   []string
	dropped int
}

func (r *Recorder) add(msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lines) >= MaxLines {
		r.dropped++
		return
	}
	r.lines = append(r.lines, time.Now().Format("15:04:05")+" "+msg)
}

// Lines returns a snapshot of recorded lines.
func (r *Recorder) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines), len(r.lines)+1)
	copy(out, r.lines)
	if r.dropped > 0 {
		out = append(out, fmt.Sprintf("... 已省略 %d 行日志（超过 %d 行上限）", r.dropped, MaxLines))
	}
	return out
}

// String joins all lines for persistence.
func (r *Recorder) String() string {
	return strings.Join(r.Lines(), "\n")
}

var live sync.Map // taskID -> *Recorder

// Start registers a live recorder for taskID and returns a context carrying it.
func Start(ctx context.Context, taskID string) (context.Context, *Recorder) {
	r := &Recorder{}
	live.Store(taskID, r)
	return context.WithValue(ctx, ctxKey{}, r), r
}

// Finish removes the live recorder; persisted logs take over afterwards.
func Finish(taskID string) {
	live.Delete(taskID)
}

// Live returns the recorder of a running task.
func Live(taskID string) (*Recorder, bool) {
	v, ok := live.Load(taskID)
	if !ok {
		return nil, false
	}
	return v.(*Recorder), true
}

// Printf logs to the process logger and, if ctx carries a recorder, to the task log.
func Printf(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if r, ok := ctx.Value(ctxKey{}).(*Recorder); ok {
		r.add(msg)
	}
}

// Split turns persisted logs back into lines.
func Split(logs string) []string {
	if logs == "" {
		return []string{}
	}
	return strings.Split(logs, "\n")
}
