//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd

package ssdp

// setReusePort is a no-op on platforms that don't expose SO_REUSEPORT
// (notably Windows). SO_REUSEADDR is also a no-op here: Windows's
// default permissive bind semantics cover the single-iface SSDP case
// and the syscall.SetsockoptInt signature differs (syscall.Handle vs
// int), so wrapping it requires x/sys/windows and a separate code
// path. tutti accepts the multi-iface fan-out reduction on Windows;
// the user can pick a NIC explicitly with --interface if needed.
func setReusePort(_ int) error { return nil }
func setReuseAddr(_ int) error { return nil }
