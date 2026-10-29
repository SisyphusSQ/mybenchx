// Package cli implements the human and machine command contract.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"uuid"

	"github.com/SisyphusSQ/mybenchx/internal/bench"
	"github.com/SisyphusSQ/mybenchx/internal/config"
	"github.com/SisyphusSQ/mybenchx/internal/monitor"
	dbmysql "github.com/SisyphusSQ/mybenchx/internal/mysql"
	"github.com/SisyphusSQ/mybenchx/internal/report"
	"github.com/spf13/cobra"
)

// Version is set by release/build ldflags; local builds default to dev.
var Version = "dev"

type failure struct {
	code int
	kind string
	err  error
	data any
}

func (e *failure) Error() string { return e.err.Error() }

type commandSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Network     bool   `json:"network"`
	Writes      bool   `json:"writes"`
	Example     string `json:"example"`
}

var specs = []commandSpec{
	{"describe", "Discover command, configuration, output and error contracts", false, false, "mybenchx describe --output json"},
	{"version", "Report executable version", false, false, "mybenchx version --output json"},
	{"workload list", "List supported read workloads and access modes", false, false, "mybenchx workload list --output json"},
	{"config validate", "Validate effective configuration without connecting", false, false, "mybenchx config validate --config examples/read.json --output json"},
	{"config show", "Show non-secret effective configuration", false, false, "mybenchx config show --config examples/read.json --output json"},
	{"plan", "Preview selected tables, operations and effects without connecting", false, false, "mybenchx plan --config examples/read.json --output json"},
	{"doctor", "Connect and inspect server and benchmark table readiness without writing", true, false, "mybenchx doctor --config .local/cases/example.json --output json"},
	{"prepare", "Create owned InnoDB tables and fill rows; requires --execute", true, true, "mybenchx prepare --config examples/read.json --execute --output json"},
	{"run", "Run bounded workload; write workers require --execute", true, true, "mybenchx run --config examples/read.json --duration 60s --output ndjson"},
	{"cleanup", "Drop only selected schema-v1 benchmark tables; requires --execute", true, true, "mybenchx cleanup --config examples/read.json --execute --output json"},
}

// Execute returns a documented process exit code without calling os.Exit.
func Execute(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	format := "text"
	if s, ok := os.LookupEnv("MYBENCHX_OUTPUT"); ok {
		format = s
	}
	// Discover output before Cobra parsing so parse failures obey the protocol.
	for i, s := range args {
		if value, ok := strings.CutPrefix(s, "--output="); ok {
			format = value
		}
		if s == "--output" && i+1 < len(args) {
			format = args[i+1]
		}
	}
	writer := report.Writer{Out: out, Format: format, RunID: uuid.New().String(), Command: "mybenchx"}
	if format != "text" && format != "json" && format != "ndjson" {
		writer.Format = "json"
		if writer.Emit("result", "failed", nil, &report.Failure{Code: "invalid_argument", Message: "output must be text, json or ndjson"}) != nil {
			return 5
		}
		return 2
	}
	var path string
	var execute bool
	root := &cobra.Command{Use: "mybenchx", Short: "Reproducible MySQL workloads for humans and automation", SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs}
	root.SetOut(out)
	root.SetErr(diagnostic)
	root.SetArgs(args)
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(*cobra.Command, error) error { return errors.New("invalid flag or value; use --help or describe") })
	root.PersistentFlags().StringVar(&path, "config", "", "Non-secret JSON configuration file")
	root.PersistentFlags().StringVar(&format, "output", format, "Output protocol: text, json or ndjson")
	root.PersistentFlags().BoolVar(&execute, "execute", false, "Explicitly permit requested database writes")
	for _, f := range config.Fields() {
		help := f.Description + " [" + f.Env + "]"
		switch value := f.Default.(type) {
		case int:
			root.PersistentFlags().Int(f.Flag, value, help)
		case uint64:
			root.PersistentFlags().Uint64(f.Flag, value, help)
		case string:
			root.PersistentFlags().String(f.Flag, value, help)
		}
	}
	load := func(cmd *cobra.Command) (config.Config, error) {
		overrides := map[string]string{}
		for _, f := range config.Fields() {
			if cmd.Flags().Changed(f.Flag) {
				overrides[f.Name] = cmd.Flags().Lookup(f.Flag).Value.String()
			}
		}
		c, err := config.Load(path, os.LookupEnv, overrides)
		if err != nil {
			return c, &failure{2, "invalid_config", err, nil}
		}
		return c, nil
	}
	emit := func(data any) error {
		if err := writer.Emit("result", "completed", data, nil); err != nil {
			return &failure{5, "output_error", errors.New("cannot write result"), nil}
		}
		return nil
	}
	for _, spec := range specs {
		var parent = root
		parts := strings.Fields(spec.Name)
		if len(parts) == 2 {
			var existing *cobra.Command
			for _, cmd := range root.Commands() {
				if cmd.Name() == parts[0] {
					existing = cmd
				}
			}
			if existing == nil {
				existing = &cobra.Command{Use: parts[0], Short: "Configuration and workload discovery", RunE: func(*cobra.Command, []string) error { return errors.New("subcommand required") }}
				root.AddCommand(existing)
			}
			parent = existing
		}
		cmd := &cobra.Command{Use: parts[len(parts)-1], Short: spec.Description, Example: spec.Example, Args: cobra.NoArgs}
		cmd.RunE = func(cmd *cobra.Command, _ []string) error {
			writer.Command = spec.Name
			switch spec.Name {
			case "describe":
				return emit(map[string]any{"commands": specs, "config_fields": config.Fields(), "config_precedence": []string{"flags", "environment", "file", "defaults"}, "formats": []string{"text", "json", "ndjson"}, "events": []string{"started", "progress", "result"}, "schema_version": report.SchemaVersion, "exit_codes": map[string]int{"success": 0, "invalid_argument_or_config": 2, "connection_failed": 3, "execution_failed": 4, "output_failed": 5, "canceled": 130}, "limits": map[string]any{"workers": 4096, "tables": 1024, "rows_per_insert": 10000, "batch": 10000, "buffered_rows_per_worker": 10000}, "enums": map[string][]string{"workload": {"point", "range", "datetime", "unix"}, "access": {"random", "sequential"}, "order": {"asc", "desc"}, "tls": {"disabled", "required"}, "monitor": {"off", "ssh"}}, "write_authorization": "prepare, cleanup and run with write workers require --execute; never automatically retry failed writes", "latency": "successful unit wall time; includes begin/commit for write batches and full result consumption for reads; excludes SQL generation; logarithmic quantiles have <=2.2% upper-bound error"})
			case "version":
				return emit(map[string]string{"version": Version, "schema_version": report.SchemaVersion})
			case "workload list":
				return emit(map[string]any{"reads": []string{"point", "range", "datetime", "unix"}, "writes": []string{"insert", "update", "delete"}, "access": []string{"sequential", "random"}, "notes": "random INSERT uses random primary keys above the startup maximum; all existing-key operations use startup table bounds; holes and concurrent deletes may yield zero rows"})
			}
			c, err := load(cmd)
			if err != nil {
				return err
			}
			if spec.Name == "config validate" || spec.Name == "config show" {
				return emit(c)
			}
			tables := make([]string, c.Tables)
			for i := range c.Tables {
				tables[i] = c.Table(i)
			}
			if spec.Name == "plan" {
				return emit(map[string]any{"config": c, "tables": tables, "prepare_rows": int64(c.Tables) * int64(c.Rows), "run_writes": c.Writes(), "cleanup_drops": tables, "connected": false, "requires_execute": []string{"prepare", "cleanup", "run with write workers"}})
			}
			if spec.Name == "run" && c.Workers() == 0 {
				return &failure{2, "invalid_config", errors.New("run requires at least one worker"), nil}
			}
			if (spec.Name == "prepare" || spec.Name == "cleanup" || (spec.Name == "run" && c.Writes())) && !execute {
				return &failure{2, "execution_not_authorized", errors.New("write operation requires --execute; inspect plan first"), nil}
			}
			if ctx.Err() != nil {
				return &failure{130, "canceled", errors.New("operation canceled"), nil}
			}
			db, err := dbmysql.Open(ctx, c)
			if err != nil {
				return &failure{3, "connection_failed", err, nil}
			}
			defer db.Close()
			switch spec.Name {
			case "doctor":
				var version string
				call, cancel := context.WithTimeout(ctx, c.TimeoutValue())
				err = db.QueryRowContext(call, "SELECT VERSION()").Scan(&version)
				cancel()
				if err != nil {
					return &failure{3, "connection_failed", dbmysql.SafeError("inspect version", err), nil}
				}
				tableErr := dbmysql.CheckTables(ctx, db, c)
				state := "ready"
				if tableErr != nil {
					state = tableErr.Error()
				}
				return emit(map[string]any{"server_version": version, "tables_ready": tableErr == nil, "table_status": state, "config": c})
			case "prepare", "cleanup":
				var result dbmysql.Lifecycle
				if spec.Name == "prepare" {
					result, err = dbmysql.Prepare(ctx, db, c)
				} else {
					result, err = dbmysql.Cleanup(ctx, db, c)
				}
				if err != nil {
					return operationFailure(ctx, err, result)
				}
				return emit(result)
			case "run":
				sample := monitor.Sample{Status: "disabled"}
				var collector *monitor.Collector
				if c.Monitor == "ssh" {
					collector, err = monitor.Open(ctx, c)
					if err != nil {
						sample = monitor.Sample{Status: "unavailable", Error: err.Error()}
					} else {
						defer collector.Close()
						sample = collector.Sample(ctx)
					}
				}
				if err = writer.Emit("started", "running", map[string]any{"config": c, "monitor": sample}, nil); err != nil {
					return &failure{5, "output_error", errors.New("cannot write start event"), nil}
				}
				var progressErr error
				result, err := bench.Run(ctx, db, c, func(s bench.Snapshot) error {
					if collector != nil {
						sample = collector.Sample(ctx)
					}
					progressErr = writer.Emit("progress", "running", map[string]any{"metrics": s, "monitor": sample}, nil)
					return progressErr
				})
				data := map[string]any{"config": c, "metrics": result, "monitor": sample}
				if progressErr != nil {
					return &failure{5, "output_error", errors.New("cannot write progress"), data}
				}
				if err != nil {
					return operationFailure(ctx, err, data)
				}
				return emit(data)
			}
			return nil
		}
		parent.AddCommand(cmd)
	}
	root.RunE = func(*cobra.Command, []string) error { return errors.New("command required; use --help or describe") }
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	var f *failure
	if found, ok := errors.AsType[*failure](err); ok {
		f = found
	} else {
		f = &failure{2, "invalid_argument", errors.New("invalid command, arguments or flags; use --help or describe"), nil}
	}
	status := "failed"
	if f.code == 130 {
		status = "canceled"
	}
	if err = writer.Emit("result", status, f.data, &report.Failure{Code: f.kind, Message: f.err.Error()}); err != nil {
		fmt.Fprintln(diagnostic, "cannot write result")
		return 5
	}
	return f.code
}

func operationFailure(ctx context.Context, err error, data any) error {
	if ctx.Err() != nil {
		return &failure{130, "canceled", errors.New("operation canceled; inspect partial result"), data}
	}
	return &failure{4, "execution_failed", err, data}
}
