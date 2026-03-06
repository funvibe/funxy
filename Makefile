.PHONY: build test install clean fmt help

# Funxy Makefile

build:
	CGO_ENABLED=0 go build -trimpath -o funxy ./cmd/funxy

test:
	go test ./... -race

install:
	go install ./cmd/funxy

clean:
	rm -f funxy

help:
	@echo "Funxy build targets:"
	@echo "  make build   - Build funxy binary"
	@echo "  make test    - Run tests"
	@echo "  make install - Install to GOPATH/bin"
	@echo "  make clean   - Remove binary"
