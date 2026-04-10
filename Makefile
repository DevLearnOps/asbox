BINARY  := asbox
PREFIX  := $(HOME)/.local

.PHONY: build install uninstall clean test test-unit test-integration test-ci

build:
	go build -o $(BINARY) .

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

test-unit:
	go test -short ./...

test-integration:
	go test -v ./integration/...

test-ci:
	go vet ./... && go test -short ./... && go test -v -count=1 ./integration/...
