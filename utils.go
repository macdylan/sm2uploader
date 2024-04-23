package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/macdylan/SMFix/fix"
)

type empty struct{}

func humanReadableSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

var reFilename = regexp.MustCompile(`^[\.\/\\~]+`)

func normalizedFilename(filename string) string {
	return reFilename.ReplaceAllString(filename, "")
}

/*
func postProcessFile(file_path string) (out []byte, err error) {
	var r *os.File
	if r, err = os.Open(file_path); err != nil {
		return
	}
	defer r.Close()
	return postProcess(r)
}
*/

func postProcess(r io.Reader) (out []byte, err error) {
	var (
		isFixed = false
		nl      = []byte("\n")
		headers = [][]byte{}
		gcodes  = []*fix.GcodeBlock{}
		sc      = bufio.NewScanner(r)
	)
	for sc.Scan() {
		line := sc.Text()
		if !isFixed && strings.HasPrefix(line, "; Postprocessed by smfix") {
			isFixed = true
		}

		g, err := fix.ParseGcodeBlock(line)
		if err == nil {
			if g.Is("G4") {
				var s int
				if err := g.GetParam('S', &s); err == nil && s == 0 {
					continue
				}
			}
			gcodes = append(gcodes, g)
			continue
		}
		if err != fix.ErrEmptyString {
			return nil, err
		}
	}

	if !isFixed {
		funcs := []fix.GcodeModifier{}

		if !noTrim {
			// funcs = append(funcs, fix.GcodeTrimLines)
		}
		if !noShutoff {
			funcs = append(funcs, fix.GcodeFixShutoff)
		}
		if !noPreheat {
			funcs = append(funcs, fix.GcodeFixPreheat)
		}
		if !noReplaceTool {
			funcs = append(funcs, fix.GcodeReplaceToolNum)
		}
		if !noReinforceTower {
			funcs = append(funcs, fix.GcodeReinforceTower)
		}

		funcs = append(funcs, fix.GcodeFixOrcaToolUnload)

		for _, fn := range funcs {
			gcodes = fn(gcodes)
		}

		if headers, err = fix.ExtractHeader(gcodes); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer

	for _, h := range headers {
		buf.Write(h)
		buf.Write(nl)
	}

	for _, gcode := range gcodes {
		buf.WriteString(gcode.String())
		buf.Write(nl)
	}
	return buf.Bytes(), nil
}

func shouldBeFix(fpath string) bool {
	ext := strings.ToLower(filepath.Ext(fpath))
	return SmFixExtensions[ext]
}
