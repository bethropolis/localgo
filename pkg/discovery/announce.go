package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
)

// SendDiscoveryAnnouncement sends a multicast announcement
func (md *MulticastDiscovery) SendDiscoveryAnnouncement() error {
	announcementDto := md.dto
	announcementDto.Announce = true

	data, err := json.Marshal(announcementDto)
	if err != nil {
		return fmt.Errorf("failed to marshal announcement: %w", err)
	}

	addr, err := net.ResolveUDPAddr("udp4", md.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	var localAddr *net.UDPAddr
	if md.config.InterfaceName != "" {
		iface, err := net.InterfaceByName(md.config.InterfaceName)
		if err == nil {
			addrs, err := iface.Addrs()
			if err == nil {
				for _, a := range addrs {
					if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
						localAddr = &net.UDPAddr{IP: ipnet.IP}
						break
					}
				}
			}
		}
	}

	conn, err := net.DialUDP("udp4", localAddr, addr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send multicast announcement: %w", err)
	}

	md.logger.Debugf("Sent multicast announcement as %s (fingerprint: %s) to %s",
		md.dto.Alias, getShortFingerprint(md.dto.Fingerprint), md.config.MulticastAddr)
	return nil
}

// SendDiscoveryResponse sends a response to a specific address
func (md *MulticastDiscovery) SendDiscoveryResponse(targetAddr *net.UDPAddr, targetDevice *model.Device) error {
	// 1. Try HTTP Response first
	if md.httpDiscoverer != nil && targetDevice != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		scheme := "http"
		if targetDevice.Protocol == model.ProtocolTypeHTTPS {
			scheme = "https"
		}

		_, err := md.httpDiscoverer.RegisterWithDevice(ctx, net.ParseIP(targetDevice.IP), targetDevice.Port, scheme)
		if err == nil {
			md.logger.Debugf("Sent discovery response via HTTP to %s:%d", targetDevice.IP, targetDevice.Port)
			return nil
		}
	}

	// 2. Fallback to UDP — send via multicast so every listener sees the response
	responseDto := md.dto
	responseDto.Announce = false

	data, err := json.Marshal(responseDto)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	respAddr, err := net.ResolveUDPAddr("udp4", md.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, respAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send discovery response: %w", err)
	}

	md.logger.Debugf("Sent discovery response via multicast to %s", md.config.MulticastAddr)
	return nil
}

func (md *MulticastDiscovery) listenLoop(ctx context.Context, conn net.PacketConn) {
	buffer := make([]byte, 2048)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if md.closed.Load() {
			return
		}

		if err := conn.SetReadDeadline(time.Now().Add(md.config.ListenTimeout)); err != nil {
			md.logger.Warnf("Failed to set read deadline: %v", err)
		}

		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			continue
		}

		if err := md.handlePacket(buffer[:n], addr); err != nil {
			md.logger.Warnf("Failed to handle multicast packet: %v", err)
		}
	}
}

func (md *MulticastDiscovery) handlePacket(data []byte, addr net.Addr) error {
	var dto model.MulticastDto
	if err := json.Unmarshal(data, &dto); err != nil {
		return fmt.Errorf("failed to unmarshal packet: %w", err)
	}

	if dto.Fingerprint == md.dto.Fingerprint {
		return nil
	}

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return fmt.Errorf("unexpected address type: %T", addr)
	}

	device := model.FromMulticastDto(dto, udpAddr.IP)

	md.logger.Debugf("Discovered raw device via multicast: %s (%s) at %s:%d",
		device.Alias, getShortFingerprint(device.Fingerprint), device.IP, device.Port)

	md.updateDevice(device)

	if dto.Announce {
		if err := md.SendDiscoveryResponse(udpAddr, device); err != nil {
			md.logger.Warnf("Failed to send discovery response: %v", err)
		}
	}

	return nil
}
