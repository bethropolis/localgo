//go:build windows

package storage

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32          = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpace = modkernel32.NewProc("GetDiskFreeSpaceExW")
)

func getAvailableBytes(path string) (uint64, error) {
	var freeBytes int64

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	r, _, err := procGetDiskFreeSpace.Call(
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
