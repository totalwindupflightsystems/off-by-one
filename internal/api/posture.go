package api

import (
	"net"
	"strings"
)

// Security posture: the HTTP API has no authentication.
//
// No handler in this package consults an Authorization header, a Bearer
// token, or an X-API-Key, and there is no requireAuth middleware anywhere
// in the chain built by Server.Handler: every route — including the write
// endpoints POST /api/v1/problems/submit, POST /api/v1/export and
// POST /api/v1/import — is answered for any client that can reach the
// socket. The access boundary is the bind address alone. The daemon
// defaults to loopback (127.0.0.1), which is what makes an
// unauthenticated API acceptable for the single-host lab deployment.
//
// LoopbackOnlyBind is the classifier behind that security posture: main
// uses it to warn at startup when an operator overrides the bind to a
// non-loopback address (e.g. --host 0.0.0.0 or an explicitly empty
// OFF_BY_ONE_HOST), because that choice exposes every route — writes
// included — to the network. Such a deployment MUST sit behind an
// authenticated reverse proxy or an equivalent network control; nothing
// in this package enforces it.

// LoopbackOnlyBind reports whether host confines the HTTP listener to a
// loopback interface. "localhost" (any case) and any address in 127.0.0.0/8
// or ::1 are loopback; the empty string (binds all interfaces), wildcard
// addresses ("0.0.0.0", "::"), routable addresses, and unresolvable
// hostnames are not — an operator-supplied name we cannot prove loopback is
// treated as exposed, fail-closed on the side of warning.
func LoopbackOnlyBind(host string) bool {
	h := strings.TrimSpace(host)
	if h == "" {
		// net.Listen with an empty host binds all interfaces (":port").
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		// A hostname other than localhost: not provably loopback.
		return false
	}
	return ip.IsLoopback()
}
