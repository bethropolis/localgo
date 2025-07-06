// Package network provides network-related utilities for LocalGo
package network

import (
	"errors"
	"fmt"
	"net"
	"strings"
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
		// Skip down, loopback, and non-multicast interfaces
		if (i.Flags&net.FlagUp) == 0 || (i.Flags&net.FlagLoopback) != 0 || (i.Flags&net.FlagMulticast) == 0 {
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
