.PHONY: app bin install build vet test clean add list remove edit help

BIN := bin/kura

app:
	go run ./cmd/app

# Produce ./bin/kura.
bin:
	go build -o $(BIN) ./cmd/app

# Install ./bin/kura into $GOBIN (or $GOPATH/bin) as `kura`.
install:
	go install ./cmd/app

# Subcommands.
# Usage:
#   make add name=foo
#   make list
#   make remove name=foo            # stdin 'yes' to confirm
#   make edit old=foo new=bar
#   make help
add:
	go run ./cmd/app add $(name)

list:
	go run ./cmd/app list

remove:
	go run ./cmd/app remove $(name)

edit:
	go run ./cmd/app edit $(old) $(new)

help:
	go run ./cmd/app help

vet:
	go vet ./...

test:
	go test ./...

# Compile-only check, no artifact left on disk.
build:
	go build -o /dev/null ./...

clean:
	rm -f app kura $(BIN)
