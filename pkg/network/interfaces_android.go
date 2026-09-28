//go:build android

// Interface enumeration fallback for Android (including Termux).
//
// The Android sandbox (SELinux) blocks the netlink route dumps that
// net.Interfaces()/net.InterfaceAddrs() rely on, so enumeration falls back
// to the SIOCGIFCONF family of ioctls, which the sandbox still permits.
// This file is compiled on android only; every other platform uses the
// standard library directly (see interfaces_default.go).
package network

import (
	"errors"
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux ioctl request codes (linux/sockios.h). Values are identical on arm64.
const (
	siocGIFCONF    = 0x8912
	siocGIFFLAGS   = 0x8913
	siocGIFNETMASK = 0x891b
	siocGIFINDEX   = 0x8933
)

// ifreqSize is sizeof(struct ifreq) on 64-bit Linux: 16-byte name + 24-byte
// union (sized by struct ifmap). All supported android targets are 64-bit
// (arm64), so a fixed stride is safe.
const ifreqSize = 40

// maxIfconfSize caps the SIOCGIFCONF buffer growth below.
const maxIfconfSize = 1 << 20

// ifreq is the Linux ioctl request struct: 16-byte name + 24-byte union.
type ifreq struct {
	name  [unix.IFNAMSIZ]byte
	union [24]byte
}

// ifconf is the argument struct for SIOCGIFCONF.
type ifconf struct {
	length int32
	buf    uintptr
}

// platformInterfaceFallback enumerates interfaces via ioctls when netlink is
// unavailable.
func platformInterfaceFallback(origErr error) ([]systemInterface, error) {
	fallback, err := ioctlSystemInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %w", errors.Join(origErr, err))
	}
	return fallback, nil
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// ioctlSystemInterfaces lists interfaces and their primary IPv4 addresses via
// SIOCGIFCONF, plus per-interface flags/index/netmask ioctls.
func ioctlSystemInterfaces() ([]systemInterface, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	// SIOCGIFCONF: grow the buffer until it holds the full interface list.
	var buf []byte
	for size := 4096; ; size *= 2 {
		if size > maxIfconfSize {
			return nil, errors.New("SIOCGIFCONF interface list exceeds size limit")
		}
		buf = make([]byte, size)
		conf := ifconf{length: int32(size), buf: uintptr(unsafe.Pointer(&buf[0]))}
		if err := ioctl(uintptr(fd), siocGIFCONF, unsafe.Pointer(&conf)); err != nil {
			return nil, fmt.Errorf("SIOCGIFCONF: %w", err)
		}
		if conf.length < int32(size) {
			buf = buf[:conf.length]
			break
		}
	}

	if len(buf) == 0 {
		return nil, errors.New("SIOCGIFCONF returned no interfaces")
	}

	seen := make(map[string]bool)
	var out []systemInterface
	for off := 0; off+ifreqSize <= len(buf); off += ifreqSize {
		name := nullString(buf[off : off+unix.IFNAMSIZ])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		// Only AF_INET entries carry a usable IPv4 address here.
		family := int(buf[off+16]) | int(buf[off+17])<<8
		if family != unix.AF_INET {
			continue
		}

		si := systemInterface{}
		si.Name = name
		si.Index = ioctlIndex(fd, name)
		si.Flags = ioctlFlags(fd, name)

		ip := net.IPv4(buf[off+20], buf[off+21], buf[off+22], buf[off+23])
		mask := ioctlNetmask(fd, name)
		if mask == nil {
			mask = net.CIDRMask(24, 32)
		}
		si.addrs = append(si.addrs, &net.IPNet{IP: ip, Mask: mask})

		out = append(out, si)
	}

	if len(out) == 0 {
		return nil, errors.New("no AF_INET interfaces found via ioctl")
	}
	return out, nil
}

func nullString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func ioctlIndex(fd int, name string) int {
	var req ifreq
	copy(req.name[:], name)
	if ioctl(uintptr(fd), siocGIFINDEX, unsafe.Pointer(&req)) != nil {
		return 0
	}
	idx := uint32(req.union[0]) | uint32(req.union[1])<<8 | uint32(req.union[2])<<16 | uint32(req.union[3])<<24
	return int(int32(idx))
}

// ioctlFlags maps the Linux IFF_* bits of the named interface to net.Flags.
func ioctlFlags(fd int, name string) net.Flags {
	var req ifreq
	copy(req.name[:], name)
	if ioctl(uintptr(fd), siocGIFFLAGS, unsafe.Pointer(&req)) != nil {
		return 0
	}
	iff := uint16(req.union[0]) | uint16(req.union[1])<<8
	var f net.Flags
	if iff&0x0001 != 0 { // IFF_UP
		f |= net.FlagUp
	}
	if iff&0x0002 != 0 { // IFF_BROADCAST
		f |= net.FlagBroadcast
	}
	if iff&0x0008 != 0 { // IFF_LOOPBACK
		f |= net.FlagLoopback
	}
	if iff&0x0010 != 0 { // IFF_POINTOPOINT
		f |= net.FlagPointToPoint
	}
	if iff&0x0040 != 0 { // IFF_RUNNING
		f |= net.FlagRunning
	}
	if iff&0x1000 != 0 { // IFF_MULTICAST
		f |= net.FlagMulticast
	}
	return f
}

func ioctlNetmask(fd int, name string) net.IPMask {
	var req ifreq
	copy(req.name[:], name)
	if ioctl(uintptr(fd), siocGIFNETMASK, unsafe.Pointer(&req)) != nil {
		return nil
	}
	family := int(req.union[0]) | int(req.union[1])<<8
	if family != unix.AF_INET {
		return nil
	}
	return net.IPv4Mask(req.union[4], req.union[5], req.union[6], req.union[7])
}
