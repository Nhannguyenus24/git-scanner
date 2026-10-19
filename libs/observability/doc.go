// Package observability wires logging and the health endpoints Kubernetes
// talks to: /healthz for liveness and /readyz for readiness.
//
// The probe listener is kept separate from a service's business routes in
// spirit even when they share a port: a saturated API handler must not make a
// pod look dead to the kubelet.
//
// Files:
//
//	logger.go  structured JSON logger
//	health.go  readiness state and dependency checks
//	server.go  the HTTP listener serving the probes
package observability
