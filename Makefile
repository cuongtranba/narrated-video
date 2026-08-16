SKILL_BIN := skills/narrated-video/bin
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

# -trimpath and a pinned toolchain are what make these builds byte-reproducible,
# which is what lets CI rebuild the committed binaries and assert they match
# their source. Adding a version stamp here would break that: writing the stamp
# changes the artifact being compared. `nv version` prints a content hash.
GOFLAGS := -trimpath -ldflags=-s -ldflags=-w

.PHONY: all build binaries test check clean

all: check binaries

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/nv ./cmd/nv

binaries:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		echo "  $$os-$$arch"; \
		mkdir -p $(SKILL_BIN)/$$os-$$arch; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags="-s -w" -o $(SKILL_BIN)/$$os-$$arch/nv ./cmd/nv || exit 1; \
	done
	@chmod +x $(SKILL_BIN)/nv $(SKILL_BIN)/*/nv
	@ls -la $(SKILL_BIN)/*/nv

test:
	go test ./...

check:
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	go test ./...

clean:
	rm -rf bin $(SKILL_BIN)/*/
