package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// shutdownTimeout bounds how long StartServer's stop function waits for
// in-flight requests to drain before forcing a close.
const shutdownTimeout = 5 * time.Second

// readHeaderTimeout bounds how long a client may take to send request
// headers (gosec G112 / slowloris).
const readHeaderTimeout = 10 * time.Second

// StartServer binds a loopback-only listener on an OS-assigned unprivileged
// port (REQ-PROXY-022, REQ-PROXY-023) and serves handler on it. It returns
// the bound address, a stop function for graceful shutdown, and an error.
//
// Binding "127.0.0.1:0" rather than "0.0.0.0:0" or "[::]:0" is the loopback
// enforcement mechanism: the OS refuses connections from any other host,
// and the "0" port asks the OS for an ephemeral port, which on every
// supported platform is drawn from a range >= 1024 for an unprivileged
// process — satisfying REQ-PROXY-023 without an explicit range check.
func StartServer(handler http.Handler) (address string, stop func() error, err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("proxy server: listen: %w", err)
	}

	// ReadHeaderTimeout bounds slow-header (slowloris) requests. The daemon
	// is a machine-wide singleton shared by every project, so one stalled
	// local request would otherwise degrade every session on the box.
	// ReadTimeout/WriteTimeout are deliberately NOT set: streaming responses
	// are long-lived by design and a write deadline would truncate them.
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Serve(ln)
	}()

	stopFn := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("proxy server: shutdown: %w", err)
		}
		return nil
	}

	return ln.Addr().String(), stopFn, nil
}
