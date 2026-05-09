// Package version exposes the tutti binary version and the supported
// manifest schema version. SchemaVersion is independent of binary version
// per the README's migration policy.
package version

// Tutti is the binary version. A var (not a const) so the catalog's
// release pipeline can stamp the tag in via `-ldflags '-X
// gitlab.com/dunn.dev/tutti/internal/version.Tutti=v0.1.0'`.
//
// The default is 0.0.0-dev rather than next-tag-dev so a local build
// that escapes into a manifest is honest about not being a release.
// 0.0.0 is conventional unreleased semver; -dev says hand-built. The
// schema accepts the value (it matches the semver pattern); the
// release pipeline overrides it with $CI_COMMIT_TAG before any
// shipped binary records itself.
var Tutti = "0.0.0-dev"

// SchemaVersion is the highest manifest schema version this binary writes.
// The validator (internal/schema) accepts any historic schema version it
// has a parser for; this constant only governs what fresh captures emit.
const SchemaVersion = 1
