# Tests

All tests live here, separate from production code. The tree mirrors the source
tree, so the tests for `libs/rabbit` are in `test/unit/libs/rabbit`.

```
test/
  unit/          fast, no network, no backends     make test
    internal/    tests for internal/<pkg>
    libs/        tests for libs/<pkg>
  integration/   real Postgres / RabbitMQ,
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
make test-integration   # export POSTGRES_URL=... (and others) first
make cover              # coverage of internal/ and libs/ → coverage.html
```

## Backends for integration tests

Point the tests at a database and a vhost of their own, never at the ones a
running stack uses. They write rows and consume real queue names: a scan test on
the dev vhost competes with a running scan-worker for `scan.tasks`, and the
rabbit tests purge `enrich.tasks`.

```bash
docker exec postgres createdb -U postgres gitscanner_test
docker exec rabbitmq rabbitmqctl add_vhost gitscanner-test
docker exec rabbitmq rabbitmqctl set_permissions -p gitscanner-test admin '.*' '.*' '.*'

export POSTGRES_URL='postgresql://postgres:postgres123@localhost:5432/gitscanner_test'
export RABBITMQ_URL='amqp://admin:admin123@localhost:5672/gitscanner-test'
export RABBITMQ_TEST_URL="$RABBITMQ_URL"
make test-integration
```
