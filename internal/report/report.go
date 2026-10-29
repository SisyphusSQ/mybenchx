// Package report implements the stdout protocol shared by every command.
package report

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/SisyphusSQ/mybenchx/internal/bench"
	"github.com/SisyphusSQ/mybenchx/internal/config"
	"github.com/SisyphusSQ/mybenchx/internal/monitor"
)

// SchemaVersion identifies the machine-readable output contract.
const SchemaVersion = "1"

// Failure exposes a stable code and a credential-safe explanation.
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Envelope wraps every machine-readable event, including errors.
type Envelope struct {
	SchemaVersion string   `json:"schema_version"`
	RunID         string   `json:"run_id"`
	Event         string   `json:"event"`
	Command       string   `json:"command"`
	Status        string   `json:"status"`
	Time          string   `json:"time"`
	Data          any      `json:"data,omitempty"`
	Error         *Failure `json:"error,omitempty"`
}

// Writer serializes events from the command goroutine.
type Writer struct {
	Out     io.Writer
	Format  string
	RunID   string
	Command string
}

// Emit writes a full event; JSON mode suppresses nonfinal events.
func (w Writer) Emit(event, status string, data any, failure *Failure) error {
	if w.Format == "json" && event != "result" {
		return nil
	}
	e := Envelope{SchemaVersion, w.RunID, event, w.Command, status, time.Now().UTC().Format(time.RFC3339Nano), data, failure}
	if w.Format == "text" {
		if _, err := fmt.Fprintf(w.Out, "%s: %s (%s)\n", w.Command, status, event); err != nil {
			return err
		}
		if failure != nil {
			if _, err := fmt.Fprintf(w.Out, "%s: %s\n", failure.Code, failure.Message); err != nil {
				return err
			}
		}
		if data == nil {
			return nil
		}
		if m, ok := data.(map[string]any); ok && w.Command == "run" {
			var b strings.Builder
			if c, ok := m["config"].(config.Config); ok && event == "started" {
				fmt.Fprintf(&b, "workers=%d tables=%d read=%s access=%s duration=%s max_operations=%d\n", c.Workers(), c.Tables, c.Workload, c.Access, c.Duration, c.MaxOperations)
			}
			var s bench.Snapshot
			switch metrics := m["metrics"].(type) {
			case bench.Snapshot:
				s = metrics
			case bench.Result:
				s = metrics.Snapshot
				fmt.Fprintf(&b, "stop_reason=%s\n", metrics.Reason)
			}
			if event != "started" {
				fmt.Fprintf(&b, "elapsed=%.3fs  ops/s=%.2f  transactions/s=%.2f  attempted=%d  succeeded=%d  rows=%d\n", s.ElapsedSeconds, s.OperationsPerSecond, s.TransactionsPerSecond, s.Attempted, s.Succeeded, s.Rows)
				fmt.Fprintf(&b, "failed=%d  canceled=%d  rolled_back=%d  commit_unknown=%d\n", s.Failed, s.Canceled, s.RolledBack, s.Unknown)
				if s.Interval != nil {
					fmt.Fprintf(&b, "interval=%.3fs  interval_ops/s=%.2f  interval_transactions/s=%.2f\n", s.Interval.Seconds, s.Interval.OperationsPerSecond, s.Interval.TransactionsPerSecond)
				}
			}
			if event == "result" {
				fmt.Fprintln(&b, "operation     samples    mean_ms     min_ms     max_ms     p50_ms     p95_ms     p99_ms")
				for _, kind := range slices.Sorted(maps.Keys(s.ByOperation)) {
					l := s.ByOperation[kind].Latency
					fmt.Fprintf(&b, "%-12s %8d %10.3f %10.3f %10.3f %10.3f %10.3f %10.3f\n", kind, l.Samples, l.MeanMS, l.MinMS, l.MaxMS, l.P50MS, l.P95MS, l.P99MS)
				}
			}
			if sample, ok := m["monitor"].(monitor.Sample); ok {
				fmt.Fprintf(&b, "monitor=%s", sample.Status)
				if sample.CPUPercent != nil {
					fmt.Fprintf(&b, "  cpu=%.2f%%", *sample.CPUPercent)
				}
				if sample.AvailableMemoryMB != nil {
					fmt.Fprintf(&b, "  available_memory=%.2fMiB", *sample.AvailableMemoryMB)
				}
				if sample.ReadIOPS != nil {
					fmt.Fprintf(&b, "  disk=%s read_iops=%.2f write_iops=%.2f read=%.2fMiB/s write=%.2fMiB/s", sample.Disk, *sample.ReadIOPS, *sample.WriteIOPS, *sample.ReadMBPS, *sample.WriteMBPS)
				}
				if sample.Error != "" {
					fmt.Fprintf(&b, " (%s)", sample.Error)
				}
				fmt.Fprintln(&b)
			}
			_, err := io.WriteString(w.Out, b.String())
			return err
		}
		b, err := json.Marshal(data, jsontext.WithIndent("  "))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w.Out, string(b))
		return err
	}
	b, err := json.Marshal(&e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Out.Write(b)
	return err
}
