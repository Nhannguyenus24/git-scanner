# Tests

All tests live here, separate from production code. The tree mirrors the source
tree, so the tests for `libs/rabbit` are in `test/unit/libs/rabbit`.

```
test/
  unit/          fast, no network, no backends     make test
    internal/    tests for internal/<pkg>
    libs/        tests for libs/<pkg>
  integration/   real Postgres / RabbitMQ / Redis,
                 behind the `integration` build tag  make test-integration
```

## Rules

- **Test the exported API.** Every file is an external `<pkg>_test` package, so
  it sees exactly what a caller sees. If something can only be tested by
  reaching into unexported code, that is a sign it should be its own exported
  function or package, not a reason to move the test next to the code.
- **Mirror the path.** A test for `internal/scan/control.go` goes in
  `test/unit/internal/scan/control_test.go`, or under `test/integration/scan/`
  when it needs a real backend.
- **Integration tests skip, not fail, without a backend.** Start each file with
  `//go:build integration` and `t.Skip` when the backend's URL is unset, so
  `go test -tags=integration ./...` works on a laptop with only part of the
  infrastructure running.

## Commands

```bash
make test               # unit tests, race detector on
make test-integration   # export REDIS_URL=... (and others) first
make cover              # coverage of internal/ and libs/ → coverage.html
```
