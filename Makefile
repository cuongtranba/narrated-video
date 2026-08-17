SKILL_BIN := skills/narrated-video/bin
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

# Three flags, each removing one source of variation between machines:
#
#   -trimpath   strips the building machine's absolute paths
#   -buildid=   clears the build id, which otherwise differs between two
#               installations of the same Go version (Homebrew's and the one CI
#               downloads) and makes an otherwise identical binary compare
#               unequal — same size, different bytes
#   CGO_ENABLED=0  removes the host's C toolchain from the inputs
#
# Together with a pinned Go version they make the build reproducible across
# machines. CI rebuilds and commits `skills/narrated-video/bin/` automatically
# when source changes, so contributors do not need to run `make binaries`.
# A version stamp would break that: writing the stamp changes the artifact.
# `nv version` prints a content hash instead.

.PHONY: all build binaries test check clean

all: check

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o bin/nv ./cmd/nv

binaries:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		echo "  $$os-$$arch"; \
		mkdir -p $(SKILL_BIN)/$$os-$$arch; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags="-s -w -buildid=" -o $(SKILL_BIN)/$$os-$$arch/nv ./cmd/nv || exit 1; \
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
