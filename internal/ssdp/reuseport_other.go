//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd

package ssdp

// setReusePort is a no-op on platforms that don't expose SO_REUSEPORT
// (notably Windows). SO_REUSEADDR alone is sufficient there.
func setReusePort(_ int) error { return nil }
