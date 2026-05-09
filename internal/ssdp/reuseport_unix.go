//go:build darwin || linux || freebsd || openbsd || netbsd

package ssdp

import "golang.org/x/sys/unix"

// setReusePort enables SO_REUSEPORT on the given fd. On Linux this
// requires kernel 3.9+. On the BSD family (darwin included) it has been
// the canonical way to bind multiple sockets to the same port for years.
//
// Best-effort: a non-nil return is non-fatal at the call site. tutti
// only hits this path for multi-iface SSDP probes on machines with more
// than one IPv4-capable NIC.
//
// Uses golang.org/x/sys/unix because syscall.SO_REUSEPORT is not
// declared on Linux even though the kernel supports the option; the
// stdlib's `syscall` package only exposes it on the BSD family.
func setReusePort(fd int) error {
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
}
