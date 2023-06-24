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

.PHONY: all clean dep darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 linux-arm6 win64 win32

darwin-arm64: $(SRC)
	GOOS=darwin GOARCH=arm64 $(CMD) -o $(DIST)$(NAME)-$@ $^

darwin-amd64: $(SRC)
	GOOS=darwin GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

linux-amd64: $(SRC)
	GOOS=linux GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

linux-arm7: $(SRC)
	GOOS=linux GOARCH=arm GOARM=7 $(CMD) -o $(DIST)$(NAME)-$@ $^

linux-arm6: $(SRC)
	GOOS=linux GOARCH=arm GOARM=6 $(CMD) -o $(DIST)$(NAME)-$@ $^

win64: $(SRC)
	GOOS=windows GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@.exe $^

win32: $(SRC)
	GOOS=windows GOARCH=386 $(CMD) -o $(DIST)$(NAME)-$@.exe $^

dep: # Get the dependencies
	go mod download

all: dep darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 linux-arm6 win64 win32
	@true

all-zip: all
	for p in darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 linux-arm6 win64.exe win32.exe; do \
		zip -j $(DIST)$(NAME)-$$p.zip $(DIST)$(NAME)-$$p README.md README.zh-cn.md LICENSE; \
	done

clean:
	rm -f $(DIST)$(NAME)-*
