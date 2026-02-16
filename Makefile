BINARY = openkh
BUILD_DIR = bin
CMD = ./cmd/openkh

.PHONY: build run clean test lint

build:
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

run: build
	$(BUILD_DIR)/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

lint:
	go vet ./...
