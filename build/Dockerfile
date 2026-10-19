# One Dockerfile for all six services (plan.md §14.2).
#
# The dependency layer is shared across every image, which is the concrete payoff
# of the single-module layout in §3.1: six images, one `go mod download`.

FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG SERVICE
ARG VERSION=dev
ARG COMMIT=
RUN test -n "$SERVICE" || (echo "SERVICE build-arg is required" && exit 1)
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w \
        -X github.com/mservice/gitscanner/libs/buildinfo.version=${VERSION} \
        -X github.com/mservice/gitscanner/libs/buildinfo.commit=${COMMIT}" \
      -o /out/app ./cmd/${SERVICE}

# Default runtime: five of the six services need nothing but the binary.
FROM gcr.io/distroless/static:nonroot AS runtime
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]

# scan-worker shells out to the git binary (plan.md §1), so it cannot be
# distroless. Built with --target runtime-git.
FROM debian:stable-slim AS runtime-git
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin scanner

# Neutralise git's own extension points before any untrusted repository is
# touched (plan.md §13.3). These are belt and braces alongside the per-invocation
# flags: a repository cannot introduce a hook or an alternate transport that runs
# a command.
ENV GIT_CONFIG_NOSYSTEM=1 \
    GIT_TERMINAL_PROMPT=0 \
    GIT_CONFIG_COUNT=3 \
    GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null \
    GIT_CONFIG_KEY_1=protocol.allow GIT_CONFIG_VALUE_1=never \
    GIT_CONFIG_KEY_2=protocol.https.allow GIT_CONFIG_VALUE_2=always

COPY --from=build /out/app /app
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app"]
