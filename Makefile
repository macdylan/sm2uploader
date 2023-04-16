DIST = dist/
NAME = sm2uploader
CMD = go build -ldflags="-w -s" -trimpath
SRC = $(shell ls *.go)

darwin-arm64: $(SRC)
	GOOS=darwin GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

darwin-amd64: $(SRC)
	GOOS=darwin GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

linux-amd64: $(SRC)
	GOOS=linux GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

linux-arm7: $(SRC)
	GOOS=linux GOARCH=arm GOARM=7 $(CMD) -o $(DIST)$(NAME)-$@ $^

win64: $(SRC)
	GOOS=windows GOARCH=amd64 $(CMD) -o $(DIST)$(NAME)-$@ $^

all: darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 win64
	@true

clean:
	rm -f $(DIST)$(NAME)-*
