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

# Zip archives are derived from PLATFORMS; windows archives carry the
# binary's .exe suffix and bundle the .bat helpers.
ZIP_TARGETS := $(foreach p,$(PLATFORMS),$(DIST)$(NAME)-$(p)$(if $(findstring windows,$(p)),.exe,).zip)

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

all-zip: all $(ZIP_TARGETS)

# One pattern rule covers both archive flavors: for windows archives the
# % stem already contains ".exe", so the prerequisite resolves to the
# windows binary while non-windows stems resolve to the plain binaries.
$(DIST)$(NAME)-%.zip: $(DIST)$(NAME)-%
	@mkdir -p $(DIST)
	case "$*" in \
		*.exe) zip -j $@ $< $(EXTRA) *.bat ;; \
		*)     zip -j $@ $< $(EXTRA) ;; \
	esac

clean:
	rm -rf $(DIST)$(NAME)-*
