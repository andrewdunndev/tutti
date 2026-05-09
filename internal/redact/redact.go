// Package redact scrubs sensitive strings out of tutti capture artifacts
// before they hit disk or the manifest. It is a pure-text pass: it does
// not parse SOAP, HTTP, or URLs; it walks bytes with a small set of
// pre-compiled regular expressions plus a learned table of exact
// substrings (device IPs, client IPs) that the caller registers as it
// discovers them.
//
// The categories tracked here are the same controlled vocabulary that
// ends up in manifest.redactions (see schema/manifest.v1.json). Applied
// returns only the categories that actually replaced something during a
// Redactor's lifetime, so the manifest never claims redactions that did
// not fire.
//
// Ordering note: auth scrubbing runs before IP scrubbing. A Subsonic
// auth token may contain digit runs that look like IP-adjacent text;
// neutralising the token first means later IP heuristics see fewer
// ambiguous fragments.
package redact

import (
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// Replacement strings. These are part of tutti's external contract:
// the README documents them and CI grep-checks may assert that captures
// contain only these placeholders. Do not rename without updating both.
const (
	placeholderDeviceIP  = "<DEVICE_IP>"
	placeholderClientIP  = "<CLIENT_IP>"
	placeholderRFC1918IP = "<RFC1918_IP>"
	placeholderToken     = "<TOKEN>"
	placeholderSalt      = "<SALT>"
	placeholderUser      = "<USER>"
)

var (
	// authzHeaderRe matches a full HTTP-style Authorization header line.
	// Header name is case-insensitive (HTTP/1.1). The value is anything
	// up to end of line; we replace the value with <TOKEN>.
	authzHeaderRe = regexp.MustCompile(`(?i)(authorization\s*:\s*)([^\r\n]+)`)

	// subsonicUserRe matches the Subsonic u=<value> query param.
	// Value runs until the next & or end-of-string or whitespace.
	subsonicUserRe = regexp.MustCompile(`([?&])(u)=([^&\s"'<>]+)`)

	// subsonicTokenRe matches t=<value>.
	subsonicTokenRe = regexp.MustCompile(`([?&])(t)=([^&\s"'<>]+)`)

	// subsonicSaltRe matches s=<value> and salt=<value>.
	// Order matters with subsonicTokenRe / subsonicUserRe only insofar
	// as they cover disjoint param names.
	subsonicSaltRe = regexp.MustCompile(`([?&])(s|salt)=([^&\s"'<>]+)`)

	// subsonicPasswordRe matches p=<value> (plaintext password fallback).
	subsonicPasswordRe = regexp.MustCompile(`([?&])(p)=([^&\s"'<>]+)`)

	// ipv4Re finds anything that looks like an IPv4 dotted quad. We then
	// validate with net.ParseIP and check it against RFC1918 ranges.
	// The boundary is intentionally loose: IPs commonly appear next to
	// ports (10.0.0.1:1054), slashes (10.0.0.1/24), or quotes.
	ipv4Re = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)

	rfc1918Nets = func() []*net.IPNet {
		blocks := []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		}
		out := make([]*net.IPNet, 0, len(blocks))
		for _, b := range blocks {
			_, n, err := net.ParseCIDR(b)
			if err != nil {
				panic(err) // unreachable: literals above are valid CIDRs
			}
			out = append(out, n)
		}
		return out
	}()
)

// Redactor scrubs strings and bytes through a sequence of passes,
// recording which redaction categories actually fired so the manifest
// can declare them honestly.
type Redactor struct {
	mu sync.Mutex

	// applied tracks which redaction categories actually replaced
	// anything during this Redactor's lifetime.
	applied map[schema.Redaction]bool

	// table is the per-category replacement state: e.g. exact IPs to
	// scrub. The outer key is the category, the inner map is from
	// literal substring (e.g. "10.0.0.42") to placeholder.
	table map[schema.Redaction]map[string]string
}

// New returns a Redactor with empty state. It is safe for use from a
// single goroutine; concurrent calls are serialised internally.
func New() *Redactor {
	return &Redactor{
		applied: make(map[schema.Redaction]bool),
		table:   make(map[schema.Redaction]map[string]string),
	}
}

// LearnDeviceIP records that a specific IP should be replaced by
// <DEVICE_IP>. Calling it multiple times accumulates all known device
// IPs; in v0 they all collapse to a single placeholder.
func (r *Redactor) LearnDeviceIP(ip string) {
	r.learn(schema.RedactionDeviceIP, ip, placeholderDeviceIP)
}

// LearnClientIP records the local IP. It is replaced with <CLIENT_IP>.
func (r *Redactor) LearnClientIP(ip string) {
	r.learn(schema.RedactionClientIP, ip, placeholderClientIP)
}

func (r *Redactor) learn(cat schema.Redaction, key, replacement string) {
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.table[cat]
	if !ok {
		m = make(map[string]string)
		r.table[cat] = m
	}
	m[key] = replacement
}

// String redacts a string in place: it applies all learned redactions
// plus auto-detection of RFC1918 IPs, Subsonic auth params (u=, t=, s=,
// salt=, p=) inside any URL, and Authorization headers in any HTTP
// transcript line.
func (r *Redactor) String(s string) string {
	if s == "" {
		return s
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Pass 1: Authorization headers. Done first so a header value that
	// happens to embed an IP-like string (unusual but possible in some
	// proxy traces) gets neutralised before the IP pass sees it.
	s = authzHeaderRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := authzHeaderRe.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		r.applied[schema.RedactionAuthTokens] = true
		return sub[1] + placeholderToken
	})

	// Pass 2: Subsonic query params. Each is its own category so we can
	// distinguish in Applied().
	s = subsonicUserRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := subsonicUserRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		r.applied[schema.RedactionUsername] = true
		// u= is an auth param too; surfacing it under both categories
		// would be more honest, but the schema treats username as a
		// distinct category and the manifest reader can infer the rest.
		return sub[1] + sub[2] + "=" + placeholderUser
	})
	s = subsonicTokenRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := subsonicTokenRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		r.applied[schema.RedactionAuthTokens] = true
		return sub[1] + sub[2] + "=" + placeholderToken
	})
	s = subsonicSaltRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := subsonicSaltRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		r.applied[schema.RedactionSalt] = true
		return sub[1] + sub[2] + "=" + placeholderSalt
	})
	s = subsonicPasswordRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := subsonicPasswordRe.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		r.applied[schema.RedactionAuthTokens] = true
		return sub[1] + sub[2] + "=" + placeholderToken
	})

	// Pass 3: learned exact-string IPs. Order matters between client
	// and device only insofar as both are exact-string scrubs; we apply
	// device first then client. If the same literal IP was learned as
	// both, device wins (caller error, but defined behaviour).
	for _, cat := range []schema.Redaction{schema.RedactionDeviceIP, schema.RedactionClientIP} {
		for needle, repl := range r.table[cat] {
			if strings.Contains(s, needle) {
				s = strings.ReplaceAll(s, needle, repl)
				r.applied[cat] = true
			}
		}
	}

	// Pass 4: RFC1918 sweep for anything still present. Counts as
	// device-IP best-effort; we do not always know whose IP it is.
	s = ipv4Re.ReplaceAllStringFunc(s, func(m string) string {
		ip := net.ParseIP(m)
		if ip == nil {
			return m
		}
		ip4 := ip.To4()
		if ip4 == nil {
			return m
		}
		for _, n := range rfc1918Nets {
			if n.Contains(ip4) {
				r.applied[schema.RedactionDeviceIP] = true
				return placeholderRFC1918IP
			}
		}
		return m
	})

	return s
}

// Bytes is the same operation as String on a byte slice. It returns a
// new slice; the input is not mutated. A nil or zero-length input
// returns a non-nil empty slice, so callers can write the result
// straight to disk without a nil check.
func (r *Redactor) Bytes(b []byte) []byte {
	if len(b) == 0 {
		return []byte{}
	}
	return []byte(r.String(string(b)))
}

// Applied returns the sorted list of Redaction categories that
// actually replaced something during this Redactor's lifetime. The
// order is the lexical order of the underlying string values, so the
// output is stable across runs and across Go map iteration order.
func (r *Redactor) Applied() []schema.Redaction {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]schema.Redaction, 0, len(r.applied))
	for cat := range r.applied {
		out = append(out, cat)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}
