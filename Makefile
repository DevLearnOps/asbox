BINARY  := asbox
PREFIX  := $(HOME)/.local

.PHONY: build install uninstall clean test

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
