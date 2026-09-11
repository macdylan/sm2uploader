package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	Version = "dev"
)

// flag_usage prints the help text. It is hand-written (not
// flag.PrintDefaults) so options can be grouped by purpose and annotated
// with device limitations. Keep it in sync with the usage strings in
// main.go's flag definitions.
func flag_usage() {
	ex, _ := os.Executable()
	name := filepath.Base(ex)
	if name == "" {
		name = "sm2uploader"
	}

	usage := `%[1]s %[2]s — upload G-code files to Snapmaker 3D printers over WiFi
Supported: Snapmaker 2 (A150/A250/A350), J1, Artisan, U1

Usage:
  %[1]s [options] file1.gcode [file2.nc ...]

Examples:
  %[1]s model.gcode                discover printers, pick one, upload
  %[1]s -host J1V19 model.gcode    upload to printer by id/ip/hostname
  %[1]s -octoprint :8844           run as OctoPrint-compatible server
                                   for slicer integration

Options:
  Printer selection:
    -host <id|ip|hostname>   target printer (default: auto-discover on LAN)
    -knownhosts <file>       known printers list (default: hosts.yaml next to
                             the executable, env KNOWN_HOSTS)
    -timeout <duration>      printer discovery timeout (default 4s)

  Preheat / motion (not available on U1):
    -tool1 <celsius>         preheat extruder 1 (J1 only)
    -tool2 <celsius>         preheat extruder 2 (J1 only, dual-extruder)
    -bed <celsius>           preheat heated bed
    -home                    home all axes

  G-code fixing (SMFix, enabled by default for .gcode):
    multi-tool fixes mainly apply to multi-extruder printers (J1); harmless
    on single-extruder models, always skipped for U1
    -nofix                   disable SMFix
    -output <dir>            save original and fixed files to <dir>; also
                             used as spool location for large uploads

  OctoPrint server mode (blocks until interrupted):
    -octoprint <addr>        listen address, e.g. ':8844'

  Misc:
    -debug                   verbose logging

Environment variables (command-line flags take precedence):
  HOST, KNOWN_HOSTS, OCTOPRINT, TOOL1, TOOL2, BED, HOME, TIMEOUT,
  NOFIX, OUTPUT_DIR, DEBUG

https://github.com/macdylan/sm2uploader
`
	fmt.Fprintf(os.Stderr, usage, name, Version)
	// No explicit exit: flag.Parse handles it (-h -> 0, bad flag -> 2).
}
