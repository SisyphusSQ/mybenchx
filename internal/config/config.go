// Package config defines non-secret benchmark configuration.
package config

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config deliberately exposes credential references rather than values.
type Config struct {
	Host           string `json:"host" help:"MySQL host"`
	Port           int    `json:"port" help:"MySQL TCP port"`
	User           string `json:"user" help:"MySQL user"`
	Database       string `json:"database" help:"Existing benchmark database"`
	PasswordEnv    string `json:"password_env" help:"Environment variable containing MySQL password"`
	PasswordFile   string `json:"password_file" help:"Owner-only password file; exclusive with password_env"`
	TLS            string `json:"tls" help:"TLS: disabled or required (verify server certificate)"`
	TablePrefix    string `json:"table_prefix" help:"Benchmark table prefix"`
	Tables         int    `json:"tables" help:"Number of tables (1..1024)"`
	Rows           int    `json:"rows" help:"Rows per table during prepare"`
	RowsPerInsert  int    `json:"rows_per_insert" help:"Rows per INSERT (1..10000)"`
	Batch          int    `json:"batch" help:"Write statements per transaction (1..10000); 1 is autocommit"`
	ReadWorkers    int    `json:"read_workers" help:"Read workers"`
	InsertWorkers  int    `json:"insert_workers" help:"Insert workers"`
	UpdateWorkers  int    `json:"update_workers" help:"Update workers"`
	DeleteWorkers  int    `json:"delete_workers" help:"Delete workers"`
	Workload       string `json:"workload" help:"Read workload: point, range, datetime or unix"`
	Access         string `json:"access" help:"Key selection: sequential or random"`
	RangeSize      int    `json:"range_size" help:"Maximum range result rows"`
	Order          string `json:"order" help:"Range order: asc or desc"`
	Duration       string `json:"duration" help:"Run duration, e.g. 60s; 0s requires max_operations"`
	MaxOperations  uint64 `json:"max_operations" help:"Maximum SQL attempts across workers; 0 means duration only"`
	Timeout        string `json:"timeout" help:"Connection and operation timeout, e.g. 5s"`
	ReportInterval string `json:"report_interval" help:"Progress interval, e.g. 1s"`
	Seed           uint64 `json:"seed" help:"Data generation seed"`
	BaseTime       string `json:"base_time" help:"UTC RFC3339 origin of generated timestamps"`
	Monitor        string `json:"monitor" help:"Host monitoring: off or ssh (Linux /proc)"`
	SSHHost        string `json:"ssh_host" help:"SSH monitoring host"`
	SSHPort        int    `json:"ssh_port" help:"SSH port"`
	SSHUser        string `json:"ssh_user" help:"SSH user"`
	SSHKeyFile     string `json:"ssh_key_file" help:"SSH private key file"`
	SSHPasswordEnv string `json:"ssh_password_env" help:"Environment variable containing SSH password"`
	SSHKnownHosts  string `json:"ssh_known_hosts" help:"OpenSSH known_hosts file"`
	SSHDisk        string `json:"ssh_disk" help:"Optional single Linux block device, e.g. nvme0n1"`
}

// Default returns a bounded, read-only workload configuration.
func Default() Config {
	return Config{Host: "127.0.0.1", Port: 3306, User: "mybenchx", Database: "sbtest", TLS: "disabled", TablePrefix: "mybenchx_", Tables: 1, Rows: 10000, RowsPerInsert: 1, Batch: 1, ReadWorkers: 1, Workload: "point", Access: "random", RangeSize: 100, Order: "asc", Duration: "60s", Timeout: "5s", ReportInterval: "1s", Seed: 1, BaseTime: "2024-01-01T00:00:00Z", Monitor: "off", SSHPort: 22}
}

// Field is the single source for flags, environment mapping and discovery.
type Field struct {
	Name        string `json:"name"`
	Flag        string `json:"flag"`
	Env         string `json:"env"`
	Type        string `json:"type"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

// Fields describes the same fields used by the CLI and environment loader.
func Fields() []Field {
	c := Default()
	v := reflect.ValueOf(c)
	t := v.Type()
	fields := make([]Field, 0, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		name := f.Tag.Get("json")
		fields = append(fields, Field{name, strings.ReplaceAll(name, "_", "-"), "MYBENCHX_" + strings.ToUpper(name), f.Type.Kind().String(), v.Field(i).Interface(), f.Tag.Get("help")})
	}
	return fields
}

// Load rejects unknown fields, duplicate keys and trailing JSON values.
func Load(path string, lookup func(string) (string, bool), overrides map[string]string) (Config, error) {
	c := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return c, errors.New("cannot read config file")
		}
		if strings.TrimSpace(string(b)) == "null" {
			return c, errors.New("config must be an object")
		}
		if err = json.Unmarshal(b, &c, json.RejectUnknownMembers(true)); err != nil {
			return c, errors.New("invalid config JSON: check field names, types and duplicate keys")
		}
	}
	v := reflect.ValueOf(&c).Elem()
	for i, f := range Fields() {
		s, ok := lookup(f.Env)
		if override, exists := overrides[f.Name]; exists {
			s, ok = override, true
		}
		if !ok {
			continue
		}
		switch v.Field(i).Kind() {
		case reflect.String:
			v.Field(i).SetString(s)
		case reflect.Int:
			n, err := strconv.ParseInt(s, 10, 32)
			if err != nil {
				return c, fmt.Errorf("%s must be an integer", f.Name)
			}
			v.Field(i).SetInt(n)
		case reflect.Uint64:
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return c, fmt.Errorf("%s must be an unsigned integer", f.Name)
			}
			v.Field(i).SetUint(n)
		}
	}
	return c, c.Validate()
}

var identifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// Validate checks configuration without reading secrets or accessing the network.
func (c Config) Validate() error {
	if c.Host == "" || c.User == "" || c.Database == "" {
		return errors.New("host, user and database are required")
	}
	if c.Port < 1 || c.Port > 65535 || c.SSHPort < 1 || c.SSHPort > 65535 {
		return errors.New("ports must be 1..65535")
	}
	if !identifier.MatchString(c.TablePrefix) || len(c.TablePrefix) > 48 {
		return errors.New("table_prefix must be an ASCII SQL identifier of at most 48 bytes")
	}
	if c.Tables < 1 || c.Tables > 1024 {
		return errors.New("tables must be 1..1024")
	}
	if c.Rows < 0 || c.RowsPerInsert < 1 || c.RowsPerInsert > 10000 || c.Batch < 1 || c.Batch > 10000 || c.RangeSize < 1 {
		return errors.New("rows must be nonnegative; rows_per_insert and batch must be 1..10000; range_size must be positive")
	}
	if c.Batch*c.RowsPerInsert > 10000 {
		return errors.New("batch * rows_per_insert must not exceed 10000 buffered rows per worker")
	}
	for _, n := range []int{c.ReadWorkers, c.InsertWorkers, c.UpdateWorkers, c.DeleteWorkers} {
		if n < 0 || n > 4096 {
			return errors.New("each worker count must be 0..4096")
		}
	}
	if c.Workers() > 4096 {
		return errors.New("total workers exceeds supported limit 4096")
	}
	if c.Workload != "point" && c.Workload != "range" && c.Workload != "datetime" && c.Workload != "unix" {
		return errors.New("workload must be point, range, datetime or unix")
	}
	if c.Access != "random" && c.Access != "sequential" {
		return errors.New("access must be random or sequential")
	}
	if c.Order != "asc" && c.Order != "desc" {
		return errors.New("order must be asc or desc")
	}
	if c.TLS != "disabled" && c.TLS != "required" {
		return errors.New("tls must be disabled or required")
	}
	d, err := time.ParseDuration(c.Duration)
	if err != nil || d < 0 {
		return errors.New("duration must be a nonnegative Go duration")
	}
	if d == 0 && c.MaxOperations == 0 {
		return errors.New("duration or max_operations must bound the run")
	}
	for name, s := range map[string]string{"timeout": c.Timeout, "report_interval": c.ReportInterval} {
		d, err := time.ParseDuration(s)
		if err != nil || d < time.Millisecond {
			return fmt.Errorf("%s must be a duration of at least 1ms", name)
		}
	}
	if _, err = time.Parse(time.RFC3339, c.BaseTime); err != nil {
		return errors.New("base_time must be RFC3339")
	}
	if c.PasswordEnv != "" && c.PasswordFile != "" {
		return errors.New("password_env and password_file are mutually exclusive")
	}
	if c.Monitor != "off" && c.Monitor != "ssh" {
		return errors.New("monitor must be off or ssh")
	}
	if c.Monitor == "ssh" && (c.SSHHost == "" || c.SSHUser == "" || c.SSHKnownHosts == "" || (c.SSHKeyFile == "" && c.SSHPasswordEnv == "")) {
		return errors.New("ssh monitoring requires host, user, known_hosts and authentication reference")
	}
	return nil
}

func (c Config) Workers() int {
	return c.ReadWorkers + c.InsertWorkers + c.UpdateWorkers + c.DeleteWorkers
}
func (c Config) Writes() bool                { return c.InsertWorkers+c.UpdateWorkers+c.DeleteWorkers > 0 }
func (c Config) Table(i int) string          { return fmt.Sprintf("%s%d", c.TablePrefix, i) }
func (c Config) TimeoutValue() time.Duration { d, _ := time.ParseDuration(c.Timeout); return d }

// Secret reads credentials only at the connection boundary.
func Secret(env, file string) (string, error) {
	if env != "" {
		s, ok := os.LookupEnv(env)
		if !ok {
			return "", errors.New("credential environment variable is unset")
		}
		return s, nil
	}
	if file == "" {
		return "", nil
	}
	f, err := os.Open(file)
	if err != nil {
		return "", errors.New("cannot open credential file")
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return "", errors.New("credential file must be owner-only regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(b) > 65536 {
		return "", errors.New("cannot read credential file or file exceeds 64KiB")
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}
