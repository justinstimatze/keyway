Before changing matching or ordering behavior in `internal/tiers` or
`internal/hookio`, read README.md's Design section first. Several choices
there — directory-as-config instead of a manifest, longest-substring
match, global-then-project ordering owned by one binary rather than two
parallel hook registrations — were made on purpose; treat them as fixed
unless you have a specific reason to revisit one.

If you fix a bug, add a named regression test alongside it, not just a
prose fix — see `TestLoad_baseAppliesRegardlessOfModel` in
`internal/tiers/tiers_test.go` for the shape: name the test after the
property being locked in, not the bug report that found it.

Before committing: `gofmt -l .`, `go vet ./...`, `staticcheck ./...`,
`go test ./...`. CI runs the same four checks on every push.
