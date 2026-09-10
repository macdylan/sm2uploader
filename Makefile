DIST     := dist/
NAME     := sm2uploader
GIT_REF  := $(or $(GITHUB_REF_NAME),$(shell git rev-parse --short HEAD))
VERSION  := -X 'main.Version=$(GIT_REF)'
FLAGS    := -ldflags="-w -s $(VERSION)"
CMD      := go build -trimpath $(FLAGS)
SRC      := $(filter-out %_test.go,$(wildcard *.go))
EXTRA    := README.md README.zh-cn.md LICENSE

# Platform targets: <os>-<arch>[-<arm>]
# Build settings are declared as target-specific variables and consumed
# by the single build rule below.
PLATFORMS := \
	darwin-arm64 \
	darwin-amd64 \
	linux-amd64 \
	linux-arm7 \
	linux-arm6 \
	windows-amd64 \
	windows-386

darwin-arm64:           GOOS=darwin
darwin-arm64:           GOARCH=arm64
darwin-amd64:           GOOS=darwin
darwin-amd64:           GOARCH=amd64
linux-amd64:            GOOS=linux
linux-amd64:            GOARCH=amd64
linux-amd64:            CGO_ENABLED=0
linux-arm7:             GOOS=linux
linux-arm7:             GOARCH=arm
linux-arm7:             GOARM=7
linux-arm7:             CGO_ENABLED=0
linux-arm6:             GOOS=linux
linux-arm6:             GOARCH=arm
linux-arm6:             GOARM=6
linux-arm6:             CGO_ENABLED=0
windows-amd64:          GOOS=windows
windows-amd64:          GOARCH=amd64
windows-amd64:          EXT=.exe
windows-386:            GOOS=windows
windows-386:            GOARCH=386
windows-386:            EXT=.exe

# Zip archive names are derived from PLATFORMS; windows archives carry
# the binary's .exe suffix and bundle the .bat helpers.

.PHONY: all all-zip test clean dep $(PLATFORMS)

# ---- Build rules ----

$(PLATFORMS):
	@mkdir -p $(DIST)
	$(CMD) -o $(DIST)$(NAME)-$@$(EXT) $(SRC)

# ---- Meta targets ----

dep:
	go mod download

test:
	@files="$$(git ls-files '*.go')"; \
	unformatted=$$(gofmt -l $$files); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	go test -count=1 ./...

all: dep $(PLATFORMS)

# Archives are packed in one explicit loop derived from PLATFORMS (no
# implicit pattern rules: chaining an explicit binary rule into a pattern
# rule breaks on some GNU make versions during a clean CI build).
all-zip: all
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		case $$p in \
			windows-*) zip -j $(DIST)$(NAME)-$$p.exe.zip $(DIST)$(NAME)-$$p.exe $(EXTRA) *.bat ;; \
			*)         zip -j $(DIST)$(NAME)-$$p.zip     $(DIST)$(NAME)-$$p     $(EXTRA) ;; \
		esac; \
	done

clean:
	rm -rf $(DIST)$(NAME)-*
