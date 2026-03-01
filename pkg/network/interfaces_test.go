package network_test

import (
	"net"
	"testing"

	"github.com/bethropolis/localgo/pkg/network"
)

func TestGetLocalIP(t *testing.T) {
	ipStr, err := network.GetLocalIP()
	if err != nil {
		t.Skipf("No local IP found, skipping test: %v", err)
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		t.Errorf("GetLocalIP returned invalid IP string: %s", ipStr)
	}

	if ip.IsLoopback() {
		t.Errorf("GetLocalIP returned loopback IP: %s", ipStr)
	}
}

func TestGetLocalIPAddresses(t *testing.T) {
	ips, err := network.GetLocalIPAddresses()
	if err != nil {
		t.Skipf("Failed to get local IPs, skipping test: %v", err)
	}

	for _, ip := range ips {
		if ip.IsLoopback() {
			t.Errorf("GetLocalIPAddresses returned loopback IP: %v", ip)
		}
		if ip.To4() == nil {
			t.Errorf("GetLocalIPAddresses returned non-IPv4 address: %v", ip)
		}
	}
}

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name     string
		ip       net.IP
		port     int
		https    bool
		expected string
	}{
		{
			name:     "IPv4 HTTP",
			ip:       net.ParseIP("192.168.1.100"),
			port:     8080,
			https:    false,
			expected: "http://192.168.1.100:8080",
		},
		{
			name:     "IPv4 HTTPS",
			ip:       net.ParseIP("10.0.0.1"),
			port:     443,
			https:    true,
			expected: "https://10.0.0.1:443",
		},
		{
			name:     "IPv6 HTTP",
			ip:       net.ParseIP("fe80::1"),
			port:     8080,
			https:    false,
			expected: "http://[fe80::1]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := network.FormatAddress(tt.ip, tt.port, tt.https)
			if result != tt.expected {
				t.Errorf("FormatAddress() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetSubnetIPs(t *testing.T) {
	ip := net.ParseIP("192.168.1.100")
	subnetIPs := network.GetSubnetIPs(ip)

	if len(subnetIPs) != 254 {
		t.Errorf("GetSubnetIPs() returned %d IPs, want 254", len(subnetIPs))
	}

	// Verify the first and last IP in the subnet
	if subnetIPs[0].String() != "192.168.1.1" {
		t.Errorf("First IP = %v, want 192.168.1.1", subnetIPs[0])
	}
	if subnetIPs[253].String() != "192.168.1.254" {
		t.Errorf("Last IP = %v, want 192.168.1.254", subnetIPs[253])
	}

	// Test with IPv6 (should return nil as it's not supported)
	ip6 := net.ParseIP("fe80::1")
	subnetIPs6 := network.GetSubnetIPs(ip6)
	if subnetIPs6 != nil {
		t.Errorf("GetSubnetIPs() for IPv6 should return nil, got %v", subnetIPs6)
	}
}
