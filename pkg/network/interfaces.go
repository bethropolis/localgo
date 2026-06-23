// Package network provides network-related utilities for LocalGo
package network

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/jackpal/gateway"
)

// GetLocalIP returns the primary non-loopback IP address of the machine
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", errors.New("no suitable local IP address found")
}

// GetLocalIPAddresses returns a list of local IP addresses for all non-loopback interfaces
func GetLocalIPAddresses() ([]net.IP, error) {
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, i := range ifaces {
		// Skip down and loopback interfaces
		if (i.Flags&net.FlagUp) == 0 || (i.Flags&net.FlagLoopback) != 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			if ipnet, ok := addr.(*net.IPNet); ok {
				// Get the IPv4 address
				if ip = ipnet.IP.To4(); ip != nil {
					ips = append(ips, ip)
				}
			}
		}
	}
	return ips, nil
}

// GetInterfaceIPs returns all IP addresses for a specific network interface
func GetInterfaceIPs(interfaceName string) ([]string, error) {
	var ips []string

	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}

	return ips, nil
}

// FormatAddress formats an IP address and port into a URL
func FormatAddress(ip net.IP, port int, https bool) string {
	protocol := "http"
	if https {
		protocol = "https"
	}

	ipStr := ip.String()

	// Check if it's an IPv6 address
	if strings.Contains(ipStr, ":") {
		return fmt.Sprintf("%s://[%s]:%d", protocol, ipStr, port)
	}

	return fmt.Sprintf("%s://%s:%d", protocol, ipStr, port)
}

// GetPreferredOutboundIP gets the preferred IP address for outbound connections
func GetPreferredOutboundIP() (net.IP, error) {
	// Connect to a public IP (no actual connection is made)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, fmt.Errorf("failed to determine preferred outbound IP: %w", err)
	}
	defer conn.Close()

	// Get the local address
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

// ParseCIDRRange parses a CIDR notation (e.g. "192.168.1.0/24") and returns
// all usable host IPs in that range (network and broadcast addresses excluded).
func ParseCIDRRange(cidr string) ([]net.IP, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("CIDR %q is not an IPv4 range", cidr)
	}

	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones
	if hostBits < 2 || hostBits > 30 {
		return nil, fmt.Errorf("CIDR %q prefix length must be /8–/30", cidr)
	}

	base := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
	totalHosts := (1 << hostBits) - 2
	var ips []net.IP
	for i := 1; i <= totalHosts; i++ {
		addr := base + uint32(i)
		ips = append(ips, net.IPv4(byte(addr>>24), byte(addr>>16), byte(addr>>8), byte(addr)))
	}
	return ips, nil
}

// GetSubnetIPs returns all IP addresses in the same /24 subnet as the given IP
func GetSubnetIPs(ip net.IP) []net.IP {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}

	var ips []net.IP
	// Assume /24 subnet for local discovery (most common)
	// Iterate 1-254
	for i := 1; i <= 254; i++ {
		// Create a copy of the base IP
		newIP := make(net.IP, len(ip4))
		copy(newIP, ip4)
		newIP[3] = byte(i)

		// Skip if it matches the original IP (optional, but scan logic might want to include self for testing)
		// Keeping self allows "finding" the local node which confirms scan is running.
		ips = append(ips, newIP)
	}
	return ips
}

// DefaultGatewayIP returns the IP address of the default network gateway.
func DefaultGatewayIP() (net.IP, error) {
	return gateway.DiscoverGateway()
}

// PrimaryLANIP returns the local IP address on the interface that owns
// the default gateway. This is useful for prioritizing the real LAN
// subnet when scanning, rather than scanning Docker/VPN subnets.
func PrimaryLANIP() (net.IP, error) {
	return gateway.DiscoverInterface()
}
