//go:build windows

package storage

import (
	"syscall"
	"unsafe"
)

func getAvailableBytes(path string) (uint64, error) {
	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	var freeBytes int64

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	r, _, err := c.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		0,
		0,
	)
	if r == 0 {
		return 0, err
	}
	return uint64(freeBytes), nil
}
