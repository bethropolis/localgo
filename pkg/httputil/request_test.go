package httputil

import (
	"net"
	"net/http"
	"testing"
)

func TestRemoteIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "ipv4 with port", remoteAddr: "192.168.1.5:53317", want: "192.168.1.5"},
		{name: "ipv6 with port", remoteAddr: "[fd00::1]:53317", want: "fd00::1"},
		{name: "ipv6 link local with zone", remoteAddr: "[fe80::1%eth0]:53317", want: "fe80::1"},
		{name: "ipv6 escaped zone", remoteAddr: "[fe80::1%25eth0]:53317", want: "fe80::1"},
		{name: "no port", remoteAddr: "10.0.0.7", want: "10.0.0.7"},
		{name: "unparseable", remoteAddr: "not-an-address", want: ""},
		{name: "empty", remoteAddr: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoteIP(&http.Request{RemoteAddr: tt.remoteAddr})
			if tt.want == "" {
				if got != nil {
					t.Fatalf("RemoteIP(%q) = %v, want nil", tt.remoteAddr, got)
				}
				return
			}
			if got == nil || !got.Equal(net.ParseIP(tt.want)) {
				t.Fatalf("RemoteIP(%q) = %v, want %s", tt.remoteAddr, got, tt.want)
			}
		})
	}
}

func TestRemoteIP_NilRequest(t *testing.T) {
	if got := RemoteIP(nil); got != nil {
		t.Fatalf("RemoteIP(nil) = %v, want nil", got)
	}
}
