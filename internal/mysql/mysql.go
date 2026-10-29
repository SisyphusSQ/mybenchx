// Package mysql owns database connections and benchmark table lifecycle.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/SisyphusSQ/mybenchx/internal/config"
	driver "github.com/go-sql-driver/mysql"
)

// Marker identifies tables created with the current benchmark schema.
const Marker = "mybenchx:v1"

// Open constructs a bounded connection pool and checks connectivity.
func Open(ctx context.Context, c config.Config) (*sql.DB, error) {
	password, err := config.Secret(c.PasswordEnv, c.PasswordFile)
	if err != nil {
		return nil, err
	}
	d := driver.NewConfig()
	d.User = c.User
	d.Passwd = password
	d.Net = "tcp"
	d.Addr = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	d.DBName = c.Database
	d.ParseTime = true
	d.Loc = time.UTC
	d.Timeout = c.TimeoutValue()
	d.ReadTimeout = c.TimeoutValue()
	d.WriteTimeout = c.TimeoutValue()
	if c.TLS == "required" {
		d.TLSConfig = "true"
	}
	d.Params = map[string]string{"time_zone": "'+00:00'"}
	connector, err := driver.NewConnector(d)
	if err != nil {
		return nil, errors.New("invalid MySQL connection configuration")
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(max(1, c.Workers()))
	db.SetMaxIdleConns(max(1, c.Workers()))
	ping, cancel := context.WithTimeout(ctx, c.TimeoutValue())
	defer cancel()
	if err = db.PingContext(ping); err != nil {
		db.Close()
		return nil, SafeError("connect", err)
	}
	return db, nil
}

// SafeError never returns server messages, DSNs, SQL values or credentials.
func SafeError(action string, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", action, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", action, context.DeadlineExceeded)
	}
	if e, ok := errors.AsType[*driver.MySQLError](err); ok {
		return fmt.Errorf("%s: mysql error %d (SQLSTATE %s)", action, e.Number, string(e.SQLState[:]))
	}
	return fmt.Errorf("%s failed (check connectivity, permissions and server availability)", action)
}

// CheckTables verifies every exact target before destructive or workload SQL.
func CheckTables(ctx context.Context, db *sql.DB, c config.Config) error {
	for i := range c.Tables {
		var marker string
		call, cancel := context.WithTimeout(ctx, c.TimeoutValue())
		err := db.QueryRowContext(call, "SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", c.Database, c.Table(i)).Scan(&marker)
		cancel()
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("benchmark table %s is missing; run prepare", c.Table(i))
		}
		if err != nil {
			return SafeError("inspect table", err)
		}
		if marker != Marker {
			return fmt.Errorf("table %s is not owned by mybenchx schema v1", c.Table(i))
		}
	}
	return nil
}

// Lifecycle records completed table changes, including partial progress.
type Lifecycle struct {
	Tables []string `json:"tables"`
	Rows   int64    `json:"rows"`
}

// Prepare fails on existing tables and never drops user data or creates a database.
func Prepare(ctx context.Context, db *sql.DB, c config.Config) (Lifecycle, error) {
	result := Lifecycle{Tables: []string{}}
	// Check the complete set first, avoiding predictable partial DDL.
	for i := range c.Tables {
		var n int
		call, cancel := context.WithTimeout(ctx, c.TimeoutValue())
		err := db.QueryRowContext(call, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", c.Database, c.Table(i)).Scan(&n)
		cancel()
		if err != nil {
			return result, SafeError("inspect table", err)
		}
		if n != 0 {
			return result, fmt.Errorf("table %s already exists", c.Table(i))
		}
	}
	base, _ := time.Parse(time.RFC3339, c.BaseTime)
	for i := range c.Tables {
		table := c.Table(i)
		ddl := fmt.Sprintf("CREATE TABLE `%s` (id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,k BIGINT NOT NULL,c VARCHAR(120) NOT NULL,pad VARCHAR(60) NOT NULL,created_at DATETIME NOT NULL,unix_stamp BIGINT NOT NULL,KEY idx_created_at(created_at),KEY idx_unix_stamp(unix_stamp)) ENGINE=InnoDB COMMENT='%s'", table, Marker)
		call, cancel := context.WithTimeout(ctx, c.TimeoutValue())
		_, err := db.ExecContext(call, ddl)
		cancel()
		if err != nil {
			return result, SafeError("create table", err)
		}
		result.Tables = append(result.Tables, table)
		for offset := 0; offset < c.Rows; {
			count := min(1000, c.Rows-offset)
			query := "INSERT INTO `" + table + "` (id,k,c,pad,created_at,unix_stamp) VALUES "
			args := make([]any, 0, count*6)
			for n := range count {
				if n > 0 {
					query += ","
				}
				query += "(?,?,?,?,?,?)"
				id := int64(offset + n + 1)
				stamp := base.Unix() + id
				args = append(args, id, id, "benchmark", "padding", time.Unix(stamp, 0).UTC(), stamp)
			}
			call, cancel := context.WithTimeout(ctx, c.TimeoutValue())
			_, err = db.ExecContext(call, query, args...)
			cancel()
			if err != nil {
				return result, SafeError("fill table", err)
			}
			result.Rows += int64(count)
			offset += count
		}
	}
	return result, nil
}

// Cleanup validates every target before dropping the exact selected tables.
func Cleanup(ctx context.Context, db *sql.DB, c config.Config) (Lifecycle, error) {
	result := Lifecycle{Tables: []string{}}
	if err := CheckTables(ctx, db, c); err != nil {
		return result, err
	}
	for i := range c.Tables {
		call, cancel := context.WithTimeout(ctx, c.TimeoutValue())
		_, err := db.ExecContext(call, "DROP TABLE `"+c.Table(i)+"`")
		cancel()
		if err != nil {
			return result, SafeError("drop table", err)
		}
		result.Tables = append(result.Tables, c.Table(i))
	}
	return result, nil
}

// Domain is a bounded snapshot of existing keys, not an assumed int64-wide range.
type Domain struct {
	Min, Max         int64
	TimeMin, TimeMax int64
}

// Domains reads indexed key bounds before timed workload execution.
func Domains(ctx context.Context, db *sql.DB, c config.Config) ([]Domain, error) {
	if err := CheckTables(ctx, db, c); err != nil {
		return nil, err
	}
	out := make([]Domain, c.Tables)
	for i := range c.Tables {
		d := &out[i]
		var lo, hi, tlo, thi sql.NullInt64
		call, cancel := context.WithTimeout(ctx, c.TimeoutValue())
		// MIN/MAX can use the indexes; COUNT(*) would scan large InnoDB tables.
		err := db.QueryRowContext(call, "SELECT MIN(id),MAX(id),MIN(unix_stamp),MAX(unix_stamp) FROM `"+c.Table(i)+"`").Scan(&lo, &hi, &tlo, &thi)
		cancel()
		if err != nil {
			return nil, SafeError("inspect key range", err)
		}
		d.Min, d.Max, d.TimeMin, d.TimeMax = lo.Int64, hi.Int64, tlo.Int64, thi.Int64
		if (c.ReadWorkers+c.UpdateWorkers+c.DeleteWorkers) > 0 && !lo.Valid {
			return nil, fmt.Errorf("table %s is empty; prepare rows before running existing-key workloads", c.Table(i))
		}
	}
	return out, nil
}
