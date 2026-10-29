// Package workload generates SQL and values independently from execution.
package workload

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/SisyphusSQ/mybenchx/internal/config"
	dbmysql "github.com/SisyphusSQ/mybenchx/internal/mysql"
)

// Statement keeps SQL structure separate from parameter values.
type Statement struct {
	SQL  string
	Args []any
	Read bool
}

// Generator belongs to one worker; it is not shared between goroutines.
type Generator struct {
	c       config.Config
	r       *rand.Rand
	seq     uint64
	domains []dbmysql.Domain
	base    int64
}

// New creates a deterministic per-worker generator over startup key bounds.
func New(c config.Config, worker int, domains []dbmysql.Domain) *Generator {
	base, _ := time.Parse(time.RFC3339, c.BaseTime)
	return &Generator{c: c, r: rand.New(rand.NewPCG(c.Seed, uint64(worker)+1)), seq: uint64(worker), domains: domains, base: base.Unix()}
}

func (g *Generator) key(lo, hi int64) int64 {
	if hi <= lo {
		return lo
	}
	// Domains are nonnegative signed BIGINT values, validated before worker start.
	width := uint64(hi-lo) + 1
	if g.c.Access == "random" {
		return lo + int64(g.r.Uint64N(width))
	}
	return lo + int64(g.seq%width)
}

// Next generates one statement without accessing a database.
func (g *Generator) Next(kind string) (Statement, error) {
	i := int(g.seq % uint64(g.c.Tables))
	if g.c.Access == "random" {
		i = g.r.IntN(g.c.Tables)
	}
	d := g.domains[i]
	table := "`" + g.c.Table(i) + "`"
	id := g.key(d.Min, d.Max)
	g.seq++
	switch kind {
	case "read":
		s := Statement{Read: true}
		switch g.c.Workload {
		case "point":
			s.SQL = "SELECT id,k,c,pad,created_at,unix_stamp FROM " + table + " WHERE id=?"
			s.Args = []any{id}
		case "range":
			s.SQL = "SELECT id,k,c,pad,created_at,unix_stamp FROM " + table + " WHERE id>=? ORDER BY id " + strings.ToUpper(g.c.Order) + " LIMIT ?"
			if g.c.Order == "desc" {
				s.SQL = strings.Replace(s.SQL, "id>=?", "id<=?", 1)
			}
			s.Args = []any{id, g.c.RangeSize}
		case "datetime", "unix":
			stamp := g.key(d.TimeMin, d.TimeMax)
			field := "unix_stamp"
			var value any = stamp
			if g.c.Workload == "datetime" {
				field = "created_at"
				value = time.Unix(stamp, 0).UTC()
			}
			s.SQL = "SELECT id,k,c,pad,created_at,unix_stamp FROM " + table + " WHERE " + field + "=?"
			s.Args = []any{value}
		}
		return s, nil
	case "update":
		return Statement{SQL: "UPDATE " + table + " SET c=? WHERE id=?", Args: []any{g.text(120), id}}, nil
	case "delete":
		return Statement{SQL: "DELETE FROM " + table + " WHERE id=?", Args: []any{id}}, nil
	case "insert":
		cols := "k,c,pad,created_at,unix_stamp"
		row := "(?,?,?,?,?)"
		random := g.c.Access == "random"
		if random {
			cols = "id," + cols
			row = "(?,?,?,?,?,?)"
		}
		s := Statement{SQL: "INSERT INTO " + table + " (" + cols + ") VALUES " + strings.TrimSuffix(strings.Repeat(row+",", g.c.RowsPerInsert), ",")}
		for range g.c.RowsPerInsert {
			if random {
				if d.Max >= math.MaxInt64-1 {
					return s, fmt.Errorf("random insert key space exhausted")
				}
				s.Args = append(s.Args, d.Max+1+g.r.Int64N(math.MaxInt64-d.Max-1))
			}
			stamp := g.base + g.r.Int64N(max(int64(1), int64(g.c.Rows))) + 1
			s.Args = append(s.Args, g.r.Int64N(math.MaxInt64), g.text(120), g.text(60), time.Unix(stamp, 0).UTC(), stamp)
		}
		return s, nil
	}
	return Statement{}, fmt.Errorf("unknown operation kind")
}

func (g *Generator) text(n int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[g.r.IntN(len(chars))]
	}
	return string(b)
}
