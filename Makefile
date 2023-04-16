DIST = dist/
NAME = sm2uploader
ifeq "$(GITHUB_REF_NAME)" ""
    VERSION := -X 'main.Version=$(shell git rev-parse --short HEAD)'
else
	VERSION := -X 'main.Version=$(GITHUB_REF_NAME)'
endif
FLAGS = -ldflags="-w -s $(VERSION)"
CMD = go build -trimpath $(FLAGS)
SRC = $(shell ls *.go | grep -v _test.go)

.PHONY: all clean dep darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 win64

darwin-arm64: $(SRC)
	GOOS=darwin GOARCH=arm64 $(CMD) -o $(DIST)$(NAME)-$@ $^

darwin-amd64: $(SRC)
	GOOS=darwin GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

linux-amd64: $(SRC)
	GOOS=linux GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

linux-arm7: $(SRC)
	GOOS=linux GOARCH=arm GOARM=7 $(CMD) -o $(DIST)$(NAME)-$@ $^

win64: $(SRC)
	GOOS=windows GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@.exe $^

dep: # Get the dependencies
	go mod download

all: dep darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 win64
	@true

clean:
	rm -f $(DIST)$(NAME)-*
