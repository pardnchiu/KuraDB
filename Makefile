.PHONY: app build vet test clean

app:
	go run ./cmd/app

vet:
	go vet ./...

test:
	go test ./...

# Compile-only check, no artifact left on disk.
build:
	go build -o /dev/null ./...

clean:
	rm -f app
