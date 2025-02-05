# Makefile

.PHONY: all build sign clean

# The default target builds and signs the binary.
all: build sign

# Build the binary.
build:
	@mkdir -p build
	ego-go build -o ./build/connect ./cmd/

# Sign the binary (depends on build).
sign: build
	ego sign ./build/connect

# Clean up build artifacts.
clean:
	@rm -rf build
