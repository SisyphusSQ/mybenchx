// Package monitor samples optional Linux host metrics over verified SSH.
package monitor

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SisyphusSQ/mybenchx/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Sample omits unavailable values instead of substituting false zero metrics.
type Sample struct {
	Status            string   `json:"status"`
	Error             string   `json:"error,omitempty"`
	CPUPercent        *float64 `json:"cpu_percent,omitempty"`
	AvailableMemoryMB *float64 `json:"available_memory_mb,omitempty"`
	ReadIOPS          *float64 `json:"read_iops,omitempty"`
	WriteIOPS         *float64 `json:"write_iops,omitempty"`
	ReadMBPS          *float64 `json:"read_mb_per_second,omitempty"`
	WriteMBPS         *float64 `json:"write_mb_per_second,omitempty"`
	Disk              string   `json:"disk,omitempty"`
}

type raw struct {
	cpu, idle, reads, writes, readSectors, writeSectors uint64
	memory                                              uint64
	disk                                                bool
	at                                                  time.Time
}

// Collector owns one verified SSH connection and successive counter snapshots.
type Collector struct {
	client *ssh.Client
	c      config.Config
	last   *raw
}

// Open authenticates SSH and verifies the host against explicit known_hosts.
func Open(ctx context.Context, c config.Config) (*Collector, error) {
	callback, err := knownhosts.New(c.SSHKnownHosts)
	if err != nil {
		return nil, errors.New("cannot read SSH known_hosts")
	}
	var auth []ssh.AuthMethod
	if c.SSHKeyFile != "" {
		key, err := config.Secret("", c.SSHKeyFile)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey([]byte(key))
		if err != nil {
			return nil, errors.New("cannot parse SSH private key (encrypted keys are unsupported)")
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if c.SSHPasswordEnv != "" {
		s, ok := os.LookupEnv(c.SSHPasswordEnv)
		if !ok {
			return nil, errors.New("SSH credential environment variable is unset")
		}
		auth = append(auth, ssh.Password(s))
	}
	addr := net.JoinHostPort(c.SSHHost, strconv.Itoa(c.SSHPort))
	dialer := net.Dialer{Timeout: c.TimeoutValue()}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, errors.New("SSH connection failed")
	}
	conn.SetDeadline(time.Now().Add(c.TimeoutValue()))
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	cc, ch, req, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{User: c.SSHUser, Auth: auth, HostKeyCallback: callback})
	stop()
	if err != nil {
		conn.Close()
		return nil, errors.New("SSH handshake failed; check identity, authentication and known_hosts")
	}
	conn.SetDeadline(time.Time{})
	return &Collector{client: ssh.NewClient(cc, ch, req), c: c}, nil
}
func (c *Collector) Close() error { return c.client.Close() }

// Sample collects Linux metrics with a bounded, cancelable SSH operation.
func (c *Collector) Sample(ctx context.Context) Sample {
	sample := Sample{Status: "unavailable", Disk: c.c.SSHDisk}
	call, cancel := context.WithTimeout(ctx, c.c.TimeoutValue())
	defer cancel()
	abort := context.AfterFunc(call, func() { c.client.Close() })
	defer abort()
	session, err := c.client.NewSession()
	if err != nil {
		sample.Error = "cannot open SSH monitoring session"
		return sample
	}
	defer session.Close()
	stop := context.AfterFunc(call, func() { session.Close() })
	defer stop()
	// Fixed command; no configuration values are interpolated into the shell.
	b, err := session.Output("LC_ALL=C cat /proc/stat /proc/meminfo /proc/diskstats")
	if err != nil {
		sample.Error = "cannot read Linux /proc metrics"
		return sample
	}
	r, err := parse(string(b), c.c.SSHDisk)
	if err != nil {
		sample.Error = err.Error()
		return sample
	}
	r.at = time.Now()
	sample.Status = "warming_up"
	sample.AvailableMemoryMB = new(float64(r.memory) / 1024)
	if c.last != nil {
		old := c.last
		seconds := r.at.Sub(old.at).Seconds()
		if r.cpu > old.cpu && r.idle >= old.idle {
			sample.CPUPercent = new(100 * (1 - float64(r.idle-old.idle)/float64(r.cpu-old.cpu)))
			sample.Status = "available"
		}
		if r.disk && old.disk && seconds > 0 && r.reads >= old.reads && r.writes >= old.writes && r.readSectors >= old.readSectors && r.writeSectors >= old.writeSectors {
			sample.ReadIOPS = new(float64(r.reads-old.reads) / seconds)
			sample.WriteIOPS = new(float64(r.writes-old.writes) / seconds)
			sample.ReadMBPS = new(float64(r.readSectors-old.readSectors) * 512 / 1024 / 1024 / seconds)
			sample.WriteMBPS = new(float64(r.writeSectors-old.writeSectors) * 512 / 1024 / 1024 / seconds)
		}
	}
	c.last = &r
	return sample
}

func parse(s, disk string) (raw, error) {
	var r raw
	var cpu, memory bool
	for line := range strings.SplitSeq(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		number := func(i int) (uint64, error) {
			if i >= len(fields) {
				return 0, errors.New("short /proc metric row")
			}
			return strconv.ParseUint(fields[i], 10, 64)
		}
		if fields[0] == "cpu" {
			if len(fields) < 9 {
				return r, errors.New("invalid CPU counters")
			}
			for i := 1; i <= 8; i++ {
				n, err := number(i)
				if err != nil {
					return r, errors.New("invalid CPU counter")
				}
				r.cpu += n
				if i == 4 || i == 5 {
					r.idle += n
				}
			}
			cpu = true
		}
		if fields[0] == "MemAvailable:" {
			n, err := number(1)
			if err != nil {
				return r, errors.New("invalid memory counter")
			}
			r.memory = n
			memory = true
		}
		if disk != "" && len(fields) > 2 && fields[2] == disk {
			dest := []*uint64{&r.reads, &r.readSectors, &r.writes, &r.writeSectors}
			for i, col := range []int{3, 5, 7, 9} {
				n, err := number(col)
				if err != nil {
					return r, errors.New("invalid disk counters")
				}
				*dest[i] = n
			}
			r.disk = true
		}
	}
	if !cpu || !memory {
		return r, errors.New("required Linux CPU/memory metrics unavailable")
	}
	if disk != "" && !r.disk {
		return r, errors.New("selected disk metrics unavailable")
	}
	return r, nil
}
