# Owner: Alexandre
# Build tool for the TAP project — all required targets from the spec.

.PHONY: deps run-server run-client run-client-gui lint clean

deps:
	go mod download

run-server:
	go run ./cmd/server data/world.json

run-client:
	go run ./cmd/cli localhost:4242

run-client-gui:
	go run ./cmd/gui

lint:
	gofmt -l . && go vet ./...

clean:
	go clean && rm -rf bin/
