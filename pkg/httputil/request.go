package httputil

import (
	"net"
	"net/http"
	"strings"
)

// RemoteIP returns the IP address a request came from, or nil when it cannot be parsed.
//
// RemoteAddr may carry an IPv6 zone (fe80::1%eth0 on a link-local peer), which net.ParseIP rejects outright. Left
// as is, such a peer would be recorded with an empty IP, so the zone is dropped: nothing in the protocol carries it.
func RemoteIP(r *http.Request) net.IP {
	if r == nil {
		return nil
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if i := strings.LastIndexByte(host, '%'); i >= 0 {
		host = host[:i]
	}

	return net.ParseIP(host)
}
