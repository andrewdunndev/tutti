// tutti is a single binary that walks SSDP and mDNS for audio renderers
// on the local network, captures device descriptors, runs library
// decision analysis, and optionally drives an AVTransport renderer end
// to end. Output is a directory of JSON plus raw artifacts.
//
// Usage:
//   tutti capture [--drive] [--allow-empty] [--no-redact] [--interface NAME]
//   tutti validate <capture-dir>
//   tutti diff-libs --descriptor <url>
//   tutti version
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"gitlab.com/dunn.dev/tutti/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch cmd {
	case "capture":
		os.Exit(runCapture(ctx, args))
	case "validate":
		os.Exit(runValidate(ctx, args))
	case "diff-libs":
		os.Exit(runDiffLibs(ctx, args))
	case "version", "--version", "-v":
		fmt.Printf("tutti %s (schema v%d)\n", version.Tutti, version.SchemaVersion)
		os.Exit(0)
	case "help", "--help", "-h":
		usage(os.Stdout)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprintf(w, `tutti %s — LAN audio renderer probe

Usage:
  tutti capture [flags]              walk LAN, write capture-<ts>-<host>/
  tutti validate <capture-dir>       check a capture against schema v%d + vacuity rules
  tutti diff-libs --descriptor URL   show how each control-point library parses one descriptor
  tutti version
  tutti help

Common capture flags:
  --drive              run AVTransport drive against discovered renderers
  --force              ignore the "device is PLAYING" precheck (use with --drive)
  --allow-empty        accept a capture with zero SSDP and zero mDNS responses
  --no-redact          do not scrub LAN IPs / auth tokens (intended for isolated test networks)
  --interface NAME     scan only the named network interface (repeatable)
  --out DIR            write the capture to DIR instead of ./capture-<ts>-<host>/
  --contributor ID     prefix:handle, e.g. github:andrewdunndev or gitlab:andrew.dunn

See https://gitlab.com/dunn.dev/tutti for the full README.
`, version.Tutti, version.SchemaVersion)
}

// strSlice implements flag.Value for repeatable string flags.
type strSlice []string

func (s *strSlice) String() string     { return strings.Join(*s, ",") }
func (s *strSlice) Set(v string) error { *s = append(*s, v); return nil }

// parseFlagsOrDie is a tiny helper for subcommand flag parsing.
func parseFlagsOrDie(name string, args []string, fn func(*flag.FlagSet)) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fn(fs)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	return fs
}
