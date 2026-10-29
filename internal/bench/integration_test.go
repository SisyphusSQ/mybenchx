//go:build integration

package bench_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
	"uuid"

	"github.com/SisyphusSQ/mybenchx/internal/bench"
	"github.com/SisyphusSQ/mybenchx/internal/cli"
	"github.com/SisyphusSQ/mybenchx/internal/config"
	dbmysql "github.com/SisyphusSQ/mybenchx/internal/mysql"
)

// The caller explicitly selects an authorized integration target. Each suite
// owns unique tables and never starts a local database or reuses existing tables.
func TestMySQLAcceptance(t *testing.T) {
	path := os.Getenv("MYBENCHX_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("set MYBENCHX_INTEGRATION_CONFIG to an authorized private config")
	}
	for _, field := range config.Fields() {
		t.Setenv(field.Env, "")
		os.Unsetenv(field.Env)
	}
	target, err := config.Load(path, func(string) (string, bool) { return "", false }, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := config.Default()
	c.Host, c.Port, c.User, c.Database = target.Host, target.Port, target.User, target.Database
	c.PasswordEnv, c.PasswordFile, c.TLS = target.PasswordEnv, target.PasswordFile, target.TLS
	c.TablePrefix = "mbx_it_" + uuid.New().String()[:8] + "_"
	c.Tables = 2
	c.Rows = 30
	c.Duration = "0s"
	c.MaxOperations = 41
	c.ReadWorkers = 4
	db, err := dbmysql.Open(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	t.Cleanup(func() {
		for i := range c.Tables {
			var marker string
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := db.QueryRowContext(ctx, "SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", c.Database, c.Table(i)).Scan(&marker)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				t.Error("could not inspect acceptance table during cleanup")
			}
			if err == nil && marker == dbmysql.Marker {
				_, err = db.ExecContext(ctx, "DROP TABLE `"+c.Table(i)+"`")
				if err != nil {
					t.Error("could not clean owned acceptance table")
				}
			}
			cancel()
		}
	})
	reset := func() {
		t.Helper()
		if _, err := dbmysql.Prepare(t.Context(), db, c); err != nil {
			t.Fatal(err)
		}
	}
	clean := func() {
		t.Helper()
		if _, err := dbmysql.Cleanup(t.Context(), db, c); err != nil {
			t.Fatal(err)
		}
	}
	reset()
	for _, access := range []string{"sequential", "random"} {
		for _, kind := range []string{"point", "range", "datetime", "unix"} {
			t.Run(access+"_"+kind, func(t *testing.T) {
				cfg := c
				cfg.Access = access
				cfg.Workload = kind
				r, err := bench.Run(t.Context(), db, cfg, nil)
				if err != nil || r.Attempted != 41 || r.Succeeded != 41 || r.Rows < 41 || r.Reason != "max_operations" {
					t.Fatalf("result=%+v err=%v", r, err)
				}
			})
		}
	}
	t.Run("existing_prepare_is_non_destructive", func(t *testing.T) {
		if _, err := dbmysql.Prepare(t.Context(), db, c); err == nil {
			t.Fatal("overwrote existing tables")
		}
		if n := rowCount(t, db, c); n != 60 {
			t.Fatal(n)
		}
	})
	clean()
	for _, access := range []string{"sequential", "random"} {
		for _, batch := range []int{1, 3} {
			t.Run("insert_"+access+strconv.Itoa(batch), func(t *testing.T) {
				reset()
				defer clean()
				cfg := c
				cfg.ReadWorkers = 0
				cfg.InsertWorkers = 3
				cfg.MaxOperations = 17
				cfg.RowsPerInsert = 2
				cfg.Batch = batch
				cfg.Access = access
				r, err := bench.Run(t.Context(), db, cfg, nil)
				if err != nil || r.Succeeded != 17 || r.Rows != 34 || r.Transactions == 0 {
					t.Fatalf("%+v %v", r, err)
				}
				if n := rowCount(t, db, c); n != 94 {
					t.Fatalf("committed rows=%d", n)
				}
			})
		}
	}
	reset()
	t.Run("mixed", func(t *testing.T) {
		cfg := c
		cfg.ReadWorkers = 2
		cfg.InsertWorkers = 2
		cfg.UpdateWorkers = 2
		cfg.DeleteWorkers = 1
		cfg.Batch = 1
		cfg.Access = "sequential"
		cfg.MaxOperations = 100
		r, err := bench.Run(t.Context(), db, cfg, nil)
		if err != nil || r.Attempted != 100 || r.Succeeded != 100 {
			t.Fatalf("%+v %v", r, err)
		}
	})
	t.Run("duration", func(t *testing.T) {
		cfg := c
		cfg.Duration = "50ms"
		cfg.MaxOperations = 0
		r, err := bench.Run(t.Context(), db, cfg, nil)
		if err != nil || r.Reason != "duration" || r.ElapsedSeconds < .04 || r.ElapsedSeconds > 3 {
			t.Fatalf("%+v %v", r, err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		cfg := c
		cfg.Duration = "10s"
		cfg.MaxOperations = 0
		cfg.ReportInterval = "50ms"
		r, err := bench.Run(ctx, db, cfg, func(bench.Snapshot) error { cancel(); return nil })
		if err == nil || r.Reason != "canceled" {
			t.Fatalf("%+v %v", r, err)
		}
	})
	t.Run("initialization_cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		r, err := bench.Run(ctx, db, c, nil)
		if err == nil || r.Reason != "canceled" {
			t.Fatalf("missing initialization cancel reason: %+v %v", r, err)
		}
	})
	t.Run("query_timeout", func(t *testing.T) {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		if _, err = conn.ExecContext(ctx, "SELECT SLEEP(2)"); err == nil {
			t.Fatal("operation ignored deadline")
		}
	})
	t.Run("cli_ndjson", func(t *testing.T) {
		var out, diagnostic bytes.Buffer
		args := []string{"run", "--config", path, "--read-workers", "1", "--insert-workers", "0", "--update-workers", "0", "--delete-workers", "0", "--max-operations", "0", "--table-prefix", c.TablePrefix, "--tables", "2", "--duration", "200ms", "--report-interval", "30ms", "--output", "ndjson"}
		code := cli.Execute(t.Context(), args, &out, &diagnostic)
		if code != 0 {
			t.Fatalf("exit %d %s", code, out.String())
		}
		lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
		if len(lines) < 3 {
			t.Fatalf("missing progress %s", out.String())
		}
		for _, line := range lines {
			var data map[string]any
			if err := json.Unmarshal(line, &data); err != nil {
				t.Fatalf("non-JSON output %s", line)
			}
		}
	})
	clean()
	t.Run("empty_tables", func(t *testing.T) {
		cfg := c
		cfg.Rows = 0
		if _, err := dbmysql.Prepare(t.Context(), db, cfg); err != nil {
			t.Fatal(err)
		}
		defer clean()
		r, err := bench.Run(t.Context(), db, cfg, nil)
		if err == nil || r.Reason != "initialization_error" {
			t.Fatal("empty existing-key workload accepted")
		}
		cfg.ReadWorkers = 0
		cfg.InsertWorkers = 1
		cfg.MaxOperations = 1
		r, err = bench.Run(t.Context(), db, cfg, nil)
		if err != nil || r.Succeeded != 1 {
			t.Fatalf("empty insert-only workload failed: %+v %v", r, err)
		}
	})
	t.Run("ownership_guard", func(t *testing.T) {
		if _, err := db.ExecContext(t.Context(), "CREATE TABLE `"+c.Table(0)+"`(id INT)"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := db.ExecContext(t.Context(), "DROP TABLE `"+c.Table(0)+"`"); err != nil {
				t.Error("could not remove ownership-test table")
			}
		}()
		if _, err := dbmysql.Cleanup(t.Context(), db, c); err == nil {
			t.Fatal("cleanup accepted an unrelated table")
		}
		var n int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM `"+c.Table(0)+"`").Scan(&n); err != nil {
			t.Fatal("unowned table was dropped")
		}
	})
}

func rowCount(t *testing.T, db *sql.DB, c config.Config) int64 {
	t.Helper()
	var total int64
	for i := range c.Tables {
		var n int64
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM `"+c.Table(i)+"`").Scan(&n); err != nil {
			t.Fatal(err)
		}
		total += n
	}
	return total
}
