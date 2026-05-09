// Package audio embeds tutti's test-tone matrix and serves it over a
// transient local HTTP server during drive tests. Tones are generated
// by scripts/gen-tones.sh; the matrix below describes them so the
// drive package can intersect against a device's announced sink list
// without parsing tag fields out of the binaries.
package audio

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

//go:embed tones/*
var tonesFS embed.FS

// Tone describes one entry in the embedded matrix. The Title here is
// the value that ffmpeg wrote into the file's metadata at generation
// time (the "tutti embed:" prefix per the README).
type Tone struct {
	Filename string
	MIME     string
	RateHz   int
	Bits     int
	Channels int
	Title    string
	Album    string
	Artist   string
}

// Matrix is the immutable list of every tone tutti ships. Order is the
// preferred fallback chain for devices that accept multiple formats.
var Matrix = []Tone{
	{Filename: "FLAC_96k_24.flac",     MIME: "audio/flac",  RateHz: 96000,  Bits: 24, Channels: 1, Title: "tutti embed: FLAC 96k 24", Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "FLAC_44k_16.flac",     MIME: "audio/flac",  RateHz: 44100,  Bits: 16, Channels: 1, Title: "tutti embed: FLAC 44k 16", Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "WAV_PCM_192k_24.wav",  MIME: "audio/wav",   RateHz: 192000, Bits: 24, Channels: 1, Title: "tutti embed: WAV PCM 192k 24", Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "WAV_PCM_96k_24.wav",   MIME: "audio/wav",   RateHz: 96000,  Bits: 24, Channels: 1, Title: "tutti embed: WAV PCM 96k 24",  Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "WAV_PCM_48k_16.wav",   MIME: "audio/wav",   RateHz: 48000,  Bits: 16, Channels: 1, Title: "tutti embed: WAV PCM 48k 16",  Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "WAV_PCM_44k_16.wav",   MIME: "audio/wav",   RateHz: 44100,  Bits: 16, Channels: 1, Title: "tutti embed: WAV PCM 44k 16",  Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "MP3_320.mp3",          MIME: "audio/mpeg",  RateHz: 44100,  Bits: 16, Channels: 1, Title: "tutti embed: MP3 320",        Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "AAC_256.m4a",          MIME: "audio/mp4",   RateHz: 44100,  Bits: 16, Channels: 1, Title: "tutti embed: AAC 256",        Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
	{Filename: "Opus_128.opus",        MIME: "audio/opus",  RateHz: 48000,  Bits: 16, Channels: 1, Title: "tutti embed: Opus 128",       Album: "Audio Renderer Probes (embedded)", Artist: "tutti.dunn.dev"},
}

// Scenario returns a stable scenario id for a tone, suitable for the
// drive_run.scenario field in the manifest.
func (t Tone) Scenario() string {
	base := strings.TrimSuffix(t.Filename, path.Ext(t.Filename))
	return strings.ToLower(strings.ReplaceAll(base, "_", "-"))
}

// Server serves the embedded tone matrix over HTTP on a local
// interface. It is bound to a specific outbound IP so the URL handed
// to the renderer is reachable from the renderer's network position.
type Server struct {
	mu       sync.Mutex
	srv      *http.Server
	bind     string
	listener net.Listener
	hits     map[string][]net.Addr
}

// NewServer prepares an unstarted server. bindIP is the address to
// bind to (typically the LAN-facing IPv4 of the local host); if zero
// the server binds to 0.0.0.0 and the caller is responsible for
// constructing reachable URLs.
func NewServer(bindIP net.IP) *Server {
	bind := "0.0.0.0:0"
	if bindIP != nil && !bindIP.IsUnspecified() {
		bind = net.JoinHostPort(bindIP.String(), "0")
	}
	return &Server{bind: bind, hits: map[string][]net.Addr{}}
}

// Start binds, begins serving in a goroutine, and returns the bound
// addr ("ip:port"). Use BaseURL to compose tone URLs.
func (s *Server) Start() (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tones/", s.handleTone)
	l, err := net.Listen("tcp", s.bind)
	if err != nil {
		return "", fmt.Errorf("bind audio server: %w", err)
	}
	s.listener = l
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = s.srv.Serve(l) }()
	return l.Addr().String(), nil
}

// Stop shuts down the server cleanly.
func (s *Server) Stop() error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Close()
}

// BaseURL returns the http://host:port prefix for composing tone URLs.
func (s *Server) BaseURL() string {
	if s.listener == nil {
		return ""
	}
	return "http://" + s.listener.Addr().String()
}

// URLFor returns the URL the device should fetch for the given tone.
func (s *Server) URLFor(t Tone) string {
	return s.BaseURL() + "/tones/" + t.Filename
}

// HitsFor returns the source addrs that fetched a given filename
// during this server's lifetime. Used to verify art_fetch_observed
// and to confirm the device actually pulled the stream.
func (s *Server) HitsFor(filename string) []net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]net.Addr, len(s.hits[filename]))
	copy(out, s.hits[filename])
	return out
}

func (s *Server) handleTone(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/tones/")
	if name == "" || strings.Contains(name, "..") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	tone, ok := lookup(name)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data, err := fs.ReadFile(tonesFS, "tones/"+name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	s.mu.Lock()
	s.hits[name] = append(s.hits[name], addrFromRequest(r))
	s.mu.Unlock()

	w.Header().Set("Content-Type", tone.MIME)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Accept-Ranges", "bytes")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func addrFromRequest(r *http.Request) net.Addr {
	host, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	ip := net.ParseIP(host)
	p := 0
	_, _ = fmt.Sscanf(port, "%d", &p)
	return &net.TCPAddr{IP: ip, Port: p}
}

func lookup(filename string) (Tone, bool) {
	for _, t := range Matrix {
		if t.Filename == filename {
			return t, true
		}
	}
	return Tone{}, false
}

// FilterByMIMEs returns the subset of the matrix whose MIME type
// appears in the device's announced set. nil or empty input returns
// the full matrix.
func FilterByMIMEs(announced []string) []Tone {
	if len(announced) == 0 {
		return append([]Tone(nil), Matrix...)
	}
	want := map[string]bool{}
	for _, m := range announced {
		want[strings.ToLower(strings.TrimSpace(m))] = true
	}
	var out []Tone
	for _, t := range Matrix {
		if want[strings.ToLower(t.MIME)] {
			out = append(out, t)
		}
	}
	return out
}

// PickOutboundIP returns the IPv4 address the OS would use to reach
// the given target. Useful for picking the audio server's bind so
// the device can reach the URL we hand it.
func PickOutboundIP(target net.IP) (net.IP, error) {
	if target == nil {
		target = net.ParseIP("8.8.8.8")
	}
	c, err := net.Dial("udp", net.JoinHostPort(target.String(), "53"))
	if err != nil {
		return nil, fmt.Errorf("pick outbound IP toward %s: %w", target, err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP, nil
}
