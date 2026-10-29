// Package bench executes bounded, cancellable SQL workloads.
package bench

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"github.com/SisyphusSQ/mybenchx/internal/config"
	dbmysql "github.com/SisyphusSQ/mybenchx/internal/mysql"
	"github.com/SisyphusSQ/mybenchx/internal/workload"
)

var errDuration = errors.New("duration reached")

// Result carries final counters and an explicit stop reason, even on failure.
type Result struct {
	Snapshot
	Reason string `json:"reason"`
}

type budget struct {
	next atomic.Uint64
	max  uint64
}

func (b *budget) take(n uint64) uint64 {
	if b.max == 0 {
		return n
	}
	for {
		old := b.next.Load()
		if old >= b.max {
			return 0
		}
		amount := min(n, b.max-old)
		if b.next.CompareAndSwap(old, old+amount) {
			return amount
		}
	}
}

// Run excludes discovery/connection setup from elapsed time. A latency sample is
// one successful unit: a fully consumed read, autocommit statement, or write
// transaction (including begin and commit). SQL generation is outside the sample.
func Run(parent context.Context, db *sql.DB, c config.Config, progress func(Snapshot) error) (result Result, err error) {
	defer func() {
		if err != nil && result.Reason == "" {
			result.Reason = "initialization_error"
			if parent.Err() != nil {
				result.Reason = "canceled"
			}
		}
	}()
	if c.Workers() == 0 {
		return Result{}, errors.New("run requires at least one worker")
	}
	// Run exclusively owns this pool while active; workers pin one connection.
	db.SetMaxOpenConns(c.Workers())
	db.SetMaxIdleConns(c.Workers())
	domains, err := dbmysql.Domains(parent, db, c)
	if err != nil {
		return Result{}, err
	}
	for _, d := range domains {
		if d.Min < 0 || d.TimeMin < 0 {
			return Result{}, errors.New("benchmark key and timestamp domains must be nonnegative")
		}
	}
	var conns []*sql.Conn
	defer func() {
		for _, conn := range conns {
			conn.Close()
		}
	}()
	for range c.Workers() {
		ctx, cancel := context.WithTimeout(parent, c.TimeoutValue())
		conn, err := db.Conn(ctx)
		cancel()
		if err != nil {
			return Result{}, dbmysql.SafeError("acquire worker connection", err)
		}
		conns = append(conns, conn)
	}
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	duration, _ := time.ParseDuration(c.Duration)
	if duration > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeoutCause(ctx, duration, errDuration)
		defer stop()
	}
	var meters []*meter
	for _, kind := range []struct {
		name string
		n    int
	}{{"read", c.ReadWorkers}, {"insert", c.InsertWorkers}, {"update", c.UpdateWorkers}, {"delete", c.DeleteWorkers}} {
		for range kind.n {
			meters = append(meters, &meter{kind: kind.name})
		}
	}
	limits := budget{max: c.MaxOperations}
	var wg sync.WaitGroup
	start := time.Now()
	for i, m := range meters {
		wg.Go(func() {
			g := workload.New(c, i, domains)
			for ctx.Err() == nil {
				size := c.Batch
				if m.kind == "read" {
					size = 1
				}
				count := limits.take(uint64(size))
				if count == 0 {
					return
				}
				statements := make([]workload.Statement, 0, count)
				for range count {
					s, err := g.Next(m.kind)
					if err != nil {
						cancel(err)
						return
					}
					statements = append(statements, s)
				}
				opctx, stop := context.WithTimeout(ctx, c.TimeoutValue())
				t := time.Now()
				counts, err := execute(opctx, conns[i], statements)
				elapsed := time.Since(t)
				stop()
				m.record(counts, elapsed, err == nil)
				if err != nil {
					if counts.Unknown > 0 || ctx.Err() == nil {
						cancel(dbmysql.SafeError("execute "+m.kind, err))
					}
					return
				}
			}
		})
	}
	done := make(chan time.Time, 1)
	go func() { wg.Wait(); done <- time.Now() }()
	interval, _ := time.ParseDuration(c.ReportInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var outputErr error
	var previous Snapshot
	for {
		select {
		case <-ticker.C:
			if progress != nil && outputErr == nil {
				current := snapshot(meters, time.Since(start))
				seconds := current.ElapsedSeconds - previous.ElapsedSeconds
				if seconds > 0 {
					current.Interval = &Interval{seconds, float64(current.Succeeded-previous.Succeeded) / seconds, float64(current.Transactions-previous.Transactions) / seconds}
				}
				previous = current
				outputErr = progress(current)
				if outputErr != nil {
					cancel(errors.New("progress output failed"))
				}
			}
		case finished := <-done:
			r := Result{Snapshot: snapshot(meters, finished.Sub(start)), Reason: "max_operations"}
			cause := context.Cause(ctx)
			switch {
			case outputErr != nil:
				r.Reason = "output_error"
				return r, outputErr
			case parent.Err() != nil:
				r.Reason = "canceled"
				return r, parent.Err()
			case errors.Is(cause, errDuration):
				r.Reason = "duration"
				if r.Unknown > 0 || r.Failed > 0 {
					r.Reason = "error"
					return r, errors.New("database failure or unknown commit at duration limit; inspect partial counters")
				}
				return r, nil
			case cause != nil:
				r.Reason = "error"
				return r, cause
			default:
				return r, nil
			}
		}
	}
}

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func statement(ctx context.Context, e executor, s workload.Statement) (uint64, error) {
	if !s.Read {
		result, err := e.ExecContext(ctx, s.SQL, s.Args...)
		if err != nil {
			return 0, err
		}
		n, err := result.RowsAffected()
		return uint64(max(0, n)), err
	}
	rows, err := e.QueryContext(ctx, s.SQL, s.Args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var n uint64
	var id, k, stamp int64
	var c, pad string
	var created time.Time
	for rows.Next() {
		if err = rows.Scan(&id, &k, &c, &pad, &created, &stamp); err != nil {
			return 0, err
		}
		n++
	}
	return n, errors.Join(rows.Err(), rows.Close())
}

func execute(ctx context.Context, conn *sql.Conn, statements []workload.Statement) (Counts, error) {
	var counts Counts
	var e executor = conn
	var tx *sql.Tx
	if len(statements) > 1 {
		var err error
		tx, err = conn.BeginTx(ctx, nil)
		if err != nil {
			return counts, err
		}
		defer tx.Rollback()
		e = tx
	}
	var rows uint64
	for _, s := range statements {
		if err := ctx.Err(); err != nil {
			counts.RolledBack = counts.Attempted
			return counts, err
		}
		counts.Attempted++
		n, err := statement(ctx, e, s)
		if err != nil {
			_, serverFailure := errors.AsType[*driver.MySQLError](err)
			if ctx.Err() != nil && !serverFailure {
				counts.Canceled = 1
			} else {
				counts.Failed = 1
			}
			if tx != nil {
				counts.RolledBack = counts.Attempted - 1
			} else if !s.Read {
				// A server error acknowledges failure. Transport loss during an
				// autocommit write cannot establish whether the server committed.
				if _, definitive := errors.AsType[*driver.MySQLError](err); !definitive {
					counts.Unknown = 1
					counts.Canceled = 0
					counts.Failed = 0
				}
			}
			return counts, err
		}
		rows += n
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			counts.Unknown = counts.Attempted
			return counts, fmt.Errorf("commit: %w", err)
		}
	}
	counts.Succeeded = counts.Attempted
	counts.Rows = rows
	if len(statements) > 0 && !statements[0].Read {
		counts.Transactions = 1
	}
	return counts, nil
}
