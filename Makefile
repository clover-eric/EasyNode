.PHONY: build build-linux-amd64 build-linux-arm64 test run clean

build:
	go build -o dist/easynode ./cmd/easynode

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o dist/easynode-linux-amd64 ./cmd/easynode

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o dist/easynode-linux-arm64 ./cmd/easynode

test:
	go test ./...

run:
	go run ./cmd/easynode -addr :8088 -data data

clean:
	rm -rf dist/easynode dist/easynode-linux-amd64 dist/easynode-linux-arm64
