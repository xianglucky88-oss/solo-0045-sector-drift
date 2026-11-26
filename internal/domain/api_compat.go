package domain

// Compile exposes route compilation to Go-level callers. The HTTP layer uses
// Build, which also encodes the artifact.
func Compile(r Request) (Plan, error) { return compile(r) }

// CompileRoute is a descriptive alias for Compile.
func CompileRoute(r Request) (Plan, error) { return compile(r) }

// PlanRoute is another common entry-point spelling.
func PlanRoute(r Request) (Plan, error) { return compile(r) }

// AsHandoverError reports whether err carries the prefix diagnostic.
func AsHandoverError(err error) (*HandoverError, bool) {
	if err == nil {
		return nil, false
	}
	if he, ok := err.(*HandoverError); ok {
		return he, true
	}
	type unwrapper interface{ Unwrap() error }
	for {
		u, ok := err.(unwrapper)
		if !ok {
			return nil, false
		}
		err = u.Unwrap()
		if err == nil {
			return nil, false
		}
		if he, ok := err.(*HandoverError); ok {
			return he, true
		}
	}
}

// PrefixFailure is an alternate diagnostic type name.
type PrefixFailure = HandoverFailure

// BlockedReport is an alternate diagnostic type name.
type BlockedReport = HandoverFailure
