DIST     := dist/
NAME     := sm2uploader
GIT_REF  := $(or $(GITHUB_REF_NAME),$(shell git rev-parse --short HEAD))
VERSION  := -X 'main.Version=$(GIT_REF)'
FLAGS    := -ldflags="-w -s $(VERSION)"
CMD      := go build -trimpath $(FLAGS)
SRC      := $(wildcard *.go)
EXTRA    := README.md README.zh-cn.md LICENSE

# Platform targets: <os>-<arch>[-<arm>]
PLATFORMS := \
	darwin-arm64 \
	darwin-amd64 \
	linux-amd64 \
	linux-arm7 \
	linux-arm6 \
	windows-amd64 \
	windows-386

.PHONY: all all-zip clean dep $(PLATFORMS)

# ---- Build rules ----

darwin-arm64:
	GOOS=darwin  GOARCH=arm64  $(CMD) -o $(DIST)$(NAME)-$@ $(SRC)

darwin-amd64:
	GOOS=darwin  GOARCH=amd64  $(CMD) -o $(DIST)$(NAME)-$@ $(SRC)

linux-amd64:
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64  $(CMD) -o $(DIST)$(NAME)-$@ $(SRC)

linux-arm7:
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm  GOARM=7  $(CMD) -o $(DIST)$(NAME)-$@ $(SRC)

linux-arm6:
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm  GOARM=6  $(CMD) -o $(DIST)$(NAME)-$@ $(SRC)

windows-amd64:
	GOOS=windows GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@.exe $(SRC)

windows-386:
	GOOS=windows GOARCH=386   $(CMD) -o $(DIST)$(NAME)-$@.exe $(SRC)

# ---- Meta targets ----

dep:
	go mod download

all: dep $(PLATFORMS)

all-zip: all
	@for p in darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 linux-arm6 windows-amd64.exe windows-386.exe; do \
		if [ "$${p%.exe}" != "$$p" ]; then \
			zip -j $(DIST)$(NAME)-$$p.zip $(DIST)$(NAME)-$$p $(EXTRA) *.bat; \
		else \
			zip -j $(DIST)$(NAME)-$$p.zip $(DIST)$(NAME)-$$p $(EXTRA); \
		fi \
	done

clean:
	rm -rf $(DIST)$(NAME)-*
