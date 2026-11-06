# S-OSPF — build, test and demo targets.
# On Windows without `make`, use scripts/demo.ps1 or run the `go` commands below.

GO ?= go
BIN := bin

.PHONY: all build test race fuzz vet lint demo emu bench clean

all: build

build:
	$(GO) build -o $(BIN)/ ./...

test:
	$(GO) test ./... -count=1

# Race detector needs CGO (a C compiler); runs on Linux CI by default.
race:
	CGO_ENABLED=1 $(GO) test -race ./... -count=1

# Fuzz smoke: exercise each parser's seed corpus briefly.
fuzz:
	$(GO) test ./internal/packet/   -run=^$$ -fuzz=FuzzDecodeRouterLSA -fuzztime=10s
	$(GO) test ./internal/packet/   -run=^$$ -fuzz=FuzzDecodeTLVs      -fuzztime=10s
	$(GO) test ./internal/packet/   -run=^$$ -fuzz=FuzzDecodeOpaque    -fuzztime=10s
	$(GO) test ./internal/semantic/ -run=^$$ -fuzz=FuzzDecodeCapabilityLSA -fuzztime=10s

vet:
	$(GO) vet ./...

# Requires staticcheck + golangci-lint on PATH.
lint:
	-staticcheck ./...
	-golangci-lint run

# The headline end-to-end scenario: planner -> best-trusted agent -> re-route.
demo:
	$(GO) run ./cmd/sospf-demo

# Converge a sample topology and print routing tables.
emu:
	$(GO) run ./cmd/sospf-emu -topo topologies/triangle.json

# Convergence + LSDB scale numbers, and resolve latency.
bench:
	$(GO) run ./cmd/sospf-bench
	$(GO) test ./internal/semantic/ -run=^$$ -bench=Resolve -benchtime=300x

clean:
	rm -rf $(BIN)
