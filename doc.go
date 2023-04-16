package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	Version = "dev"
)

func flag_usage() {
	ex, _ := os.Executable()
	usage := `%s [options] file1.gcode file2.nc ...

%s <https://github.com/macdylan/sm2uploader>

Options:
`
	fmt.Printf(usage, filepath.Base(ex), Version)
	flag.PrintDefaults()
	os.Exit(1)
}
