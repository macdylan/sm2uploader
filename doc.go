package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func flag_usage() {
	ex, _ := os.Executable()
	usage := `%s [options] file1.gcode file2.nc ...

<https://github.com/macdylan/sm2uploader>

Options:
`
	fmt.Printf(usage, filepath.Base(ex))
	flag.PrintDefaults()
	os.Exit(1)
}
