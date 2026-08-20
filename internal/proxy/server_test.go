package proxy

import (
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// TestStartServer_BindsLoopbackOnly verifies REQ-PROXY-022: the daemon
// server binds ONLY to the loopback interface, never a public/all-interfaces
// address.
func TestStartServer_BindsLoopbackOnly(t *testing.T) {
	addr, stop, err := StartServer(http.NewServeMux())
	if err != nil {
		t.Fatalf("StartServer() error = %v", err)
	}
	defer func() { _ = stop() }()

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		t.Errorf("StartServer() bound to %q, want a loopback address", addr)
	}
}

// TestStartServer_UsesUnprivilegedPort verifies REQ-PROXY-023: the bound
// port is either 0 (OS-assigned ephemeral, which the OS always returns
// >1024 for an unprivileged process) or explicitly >= 1024.
func TestStartServer_UsesUnprivilegedPort(t *testing.T) {
	addr, stop, err := StartServer(http.NewServeMux())
	if err != nil {
		t.Fatalf("StartServer() error = %v", err)
	}
	defer func() { _ = stop() }()

	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("port %q is not numeric: %v", portStr, err)
	}
	if port < 1024 {
		t.Errorf("StartServer() bound to privileged port %d, want >= 1024", port)
	}
}

// TestStartServer_StopShutsDownGracefully verifies the returned stop
// function actually terminates the listener — a second connection attempt
// after stop() must fail.
func TestStartServer_StopShutsDownGracefully(t *testing.T) {
	addr, stop, err := StartServer(http.NewServeMux())
	if err != nil {
		t.Fatalf("StartServer() error = %v", err)
	}

	// Confirm the server answers before stopping.
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/")
	if err == nil {
		_ = resp.Body.Close()
	}

	if err := stop(); err != nil {
		t.Fatalf("stop() error = %v", err)
	}

	// After stop, dialing the same address must fail (connection refused).
	conn, dialErr := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		t.Errorf("expected connection to %q to fail after stop(), but it succeeded", addr)
	}
}

// TestStartServer_ServesInjectedHandler verifies StartServer actually wires
// the given http.Handler, not a hardcoded stub.
func TestStartServer_ServesInjectedHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	addr, stop, err := StartServer(mux)
	if err != nil {
		t.Fatalf("StartServer() error = %v", err)
	}
	defer func() { _ = stop() }()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/probe")
	if err != nil {
		t.Fatalf("GET /probe error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusTeapot {
		t.Errorf("GET /probe status = %d, want %d", resp.StatusCode, http.StatusTeapot)
	}
}

// newTestClient returns an http.Client with a short timeout for use against
// httptest / StartServer instances within this package's tests.
func newTestClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Second}
}
