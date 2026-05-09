// Package version exposes the tutti binary version and the supported
// manifest schema version. SchemaVersion is independent of binary version
// per the README's migration policy.
package version

// Tutti is the binary version. A var (not a const) so the catalog's
// release pipeline can stamp the tag in via `-ldflags '-X
// gitlab.com/dunn.dev/tutti/internal/version.Tutti=v0.1.0'`. Local
// builds without the ldflag see the development default below.
var Tutti = "0.1.0-dev"

// SchemaVersion is the highest manifest schema version this binary writes.
// The validator (internal/schema) accepts any historic schema version it
// has a parser for; this constant only governs what fresh captures emit.
const SchemaVersion = 1
