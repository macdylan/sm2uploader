PROJNAME = sm2uploader
LDFLAGS = -w -s
CMD = go build -ldflags="$(LDFLAGS)"
DIST = dist/

SRC = sm2uploader.go machine.go connector.go localstorage.go

darwin-arm64: $(SRC)
	GOOS=darwin GOARCH=arm64 \
		 $(CMD) -o $(DIST)$(PROJNAME)-$@ $^

darwin-amd64: $(SRC)
	GOOS=darwin GOARCH=amd64 \
		 $(CMD) -o $(DIST)$(PROJNAME)-$@ $^

linux-amd64: $(SRC)
	GOOS=linux GOARCH=amd64 \
		 $(CMD) -o $(DIST)$(PROJNAME)-$@ $^

linux-arm7: $(SRC)
	GOOS=linux GOARCH=arm GOARM=7 \
		 $(CMD) -o $(DIST)$(PROJNAME)-$@ $^

linux-arm6: $(SRC)
	GOOS=linux GOARCH=arm GOARM=6 \
		 $(CMD) -o $(DIST)$(PROJNAME)-$@ $^

win64: $(SRC)
	GOOS=windows GOARCH=amd64 \
		 $(CMD) -o $(DIST)$(PROJNAME)-$@.exe $^

win32: $(SRC)
	GOOS=windows GOARCH=386 \
		 $(CMD) -o $(DIST)$(PROJNAME)-$@.exe $^

all: darwin-arm64 darwin-amd64 linux-amd64 linux-arm7 linux-arm6 win64 win32
	@true

clean:
	rm -f $(DIST)$(PROJNAME)-*
