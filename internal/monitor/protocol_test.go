package monitor

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/SisyphusSQ/mybenchx/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestSSHRequiresKnownHostAndSamples(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	server := &ssh.ServerConfig{NoClientAuth: true}
	server.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var samples atomic.Uint64
	wg.Go(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer conn.Close()
				sc, channels, requests, err := ssh.NewServerConn(conn, server)
				if err != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(requests)
				for ch := range channels {
					channel, reqs, err := ch.Accept()
					if err != nil {
						return
					}
					for req := range reqs {
						if req.Type == "exec" {
							req.Reply(true, nil)
							fmt.Fprintf(channel, "cpu %d 0 20 70 0 0 0 0\nMemAvailable: 2048 kB\n", 10+samples.Add(1))
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							channel.Close()
						}
					}
				}
			})
		}
	})
	t.Cleanup(func() { listener.Close(); wg.Wait() })
	file := filepath.Join(t.TempDir(), "known_hosts")
	addr := listener.Addr().String()
	os.WriteFile(file, []byte(knownhosts.Line([]string{knownhosts.Normalize(addr)}, signer.PublicKey())+"\n"), 0600)
	host, port, _ := net.SplitHostPort(addr)
	c := config.Default()
	c.SSHHost = host
	c.SSHPort, _ = strconv.Atoi(port)
	c.SSHUser = "test"
	c.SSHKnownHosts = file
	c.SSHPasswordEnv = "MYBENCHX_SSH_TEST_PASSWORD"
	t.Setenv(c.SSHPasswordEnv, "test-only")
	collector, err := Open(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer collector.Close()
	first := collector.Sample(t.Context())
	if first.Status != "warming_up" || first.AvailableMemoryMB == nil {
		t.Fatalf("bad first sample %+v", first)
	}
	next := collector.Sample(t.Context())
	if next.Status != "available" || next.CPUPercent == nil {
		t.Fatalf("bad next sample %+v", next)
	}
	os.WriteFile(file, nil, 0600)
	if untrusted, err := Open(t.Context(), c); err == nil {
		untrusted.Close()
		t.Fatal("accepted an unknown SSH host")
	}
}
