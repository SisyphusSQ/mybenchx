package workload

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SisyphusSQ/mybenchx/internal/config"
	dbmysql "github.com/SisyphusSQ/mybenchx/internal/mysql"
)

func TestTimeWorkloadsUseSameDataDomain(t *testing.T) {
	c := config.Default()
	c.Workload = "datetime"
	domain := []dbmysql.Domain{{Min: 1, Max: 10, TimeMin: 1704067201, TimeMax: 1704067210}}
	a := New(c, 0, domain)
	c.Workload = "unix"
	b := New(c, 0, domain)
	for range 100 {
		sa, _ := a.Next("read")
		sb, _ := b.Next("read")
		if sa.Args[0].(time.Time).Unix() != sb.Args[0].(int64) {
			t.Fatal("comparison uses different timestamp distributions")
		}
	}
}

func TestSeedAndParameterizedSQL(t *testing.T) {
	c := config.Default()
	c.RowsPerInsert = 3
	d := []dbmysql.Domain{{Min: 1, Max: 10}}
	a, b := New(c, 0, d), New(c, 0, d)
	for range 20 {
		sa, err := a.Next("insert")
		if err != nil {
			t.Fatal(err)
		}
		sb, _ := b.Next("insert")
		if !reflect.DeepEqual(sa, sb) {
			t.Fatal("same seed did not reproduce input")
		}
		if strings.Count(sa.SQL, "?") != len(sa.Args) {
			t.Fatal("placeholder mismatch")
		}
	}
}
