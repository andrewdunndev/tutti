//go:build darwin || linux || freebsd || openbsd || netbsd

package ssdp

import "syscall"

// setReusePort enables SO_REUSEPORT on the given fd. On Linux this
// requires kernel 3.9+. On the BSD family (darwin included) it has been
// the canonical way to bind multiple sockets to the same port for years.
//
// Best-effort: a non-nil return is non-fatal at the call site. tutti
// only hits this path for multi-iface SSDP probes on machines with more
// than one IPv4-capable NIC.
func setReusePort(fd int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
}
