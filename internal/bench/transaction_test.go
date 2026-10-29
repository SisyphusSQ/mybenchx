package bench

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	mysql "github.com/go-sql-driver/mysql"

	"github.com/SisyphusSQ/mybenchx/internal/workload"
)

type fakeConn struct {
	calls, begins, commits, rollbacks int
	failAt                            int
	commitErr                         error
	execErr                           error
	cancel                            context.CancelFunc
}
type fakeConnector struct{ c *fakeConn }

func (f fakeConnector) Connect(context.Context) (driver.Conn, error) { return f.c, nil }
func (f fakeConnector) Driver() driver.Driver                        { return fakeDriver{f.c} }

type fakeDriver struct{ c *fakeConn }

func (f fakeDriver) Open(string) (driver.Conn, error)   { return f.c, nil }
func (f *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }
func (f *fakeConn) Close() error                        { return nil }
func (f *fakeConn) Begin() (driver.Tx, error)           { f.begins++; return fakeTx{f}, nil }
func (f *fakeConn) ExecContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	f.calls++
	if f.calls == f.failAt {
		if f.cancel != nil {
			f.cancel()
		}
		if f.execErr != nil {
			return nil, f.execErr
		}
		return nil, errors.New("execution failed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return driver.RowsAffected(2), nil
}

func TestServerFailureIsNotHiddenByCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f := &fakeConn{failAt: 1, cancel: cancel, execErr: &mysql.MySQLError{Number: 1205}}
	db := sql.OpenDB(fakeConnector{f})
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c, err := execute(ctx, conn, []workload.Statement{{SQL: "UPDATE"}})
	if err == nil || c.Failed != 1 || c.Canceled != 0 || c.Unknown != 0 {
		t.Fatalf("server failure was hidden: %+v %v", c, err)
	}
}

type fakeTx struct{ c *fakeConn }

func (f fakeTx) Commit() error   { f.c.commits++; return f.c.commitErr }
func (f fakeTx) Rollback() error { f.c.rollbacks++; return nil }

func TestTransactionAccounting(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		n, fail                                int
		commitErr                              error
		wantSuccess, wantRollback, wantUnknown uint64
		wantBegins                             int
	}{
		{"autocommit", 1, 0, nil, 1, 0, 0, 0},
		{"batch", 3, 0, nil, 3, 0, 0, 1},
		{"rollback", 3, 2, nil, 0, 1, 0, 1},
		{"commit_unknown", 3, 0, io.ErrUnexpectedEOF, 0, 0, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeConn{failAt: tc.fail, commitErr: tc.commitErr}
			db := sql.OpenDB(fakeConnector{f})
			defer db.Close()
			conn, err := db.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			statements := make([]workload.Statement, tc.n)
			for i := range statements {
				statements[i].SQL = "INSERT"
			}
			counts, err := execute(t.Context(), conn, statements)
			if counts.Succeeded != tc.wantSuccess || counts.RolledBack != tc.wantRollback || counts.Unknown != tc.wantUnknown || f.begins != tc.wantBegins {
				t.Fatalf("counts=%+v connection=%+v error=%v", counts, f, err)
			}
			if tc.fail > 0 && f.rollbacks != 1 {
				t.Fatal("failed transaction not rolled back")
			}
			if tc.wantSuccess > 0 && counts.Rows != 2*tc.wantSuccess {
				t.Fatal("wrong affected rows")
			}
		})
	}
}
