package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
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

func postProcessFile(file_path string) (out []byte, err error) {
	var r *os.File
	if r, err = os.Open(file_path); err != nil {
		return
	}
	defer r.Close()
	return postProcess(r)
}

func postProcess(r io.Reader) (out []byte, err error) {
	header, errfix := fix.ExtractHeader(r)

	if h, ok := r.(io.ReadSeeker); ok {
		h.Seek(0, 0)
	}

	gcodes := make([]string, 0, fix.Params.TotalLines+len(header)+128)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		gcodes = append(gcodes, sc.Text())
	}
	if err = sc.Err(); err != nil {
		return nil, err
	}

	if errfix == nil {
		if !noTrim {
			gcodes = fix.GcodeTrimLines(gcodes)
		}
		if !noShutoff {
			gcodes = fix.GcodeFixShutoff(gcodes)
		}
		if !noPreheat {
			gcodes = fix.GcodeFixPreheat(gcodes)
		}
		if !noReinforceTower {
			gcodes = fix.GcodeReinforceTower(gcodes)
		}
	}

	out = []byte(strings.Join(gcodes, "\n"))

	if len(header) > 25 {
		return append(bytes.Join(header, []byte("\n")), out...), nil
	} else {
		return out, errfix
	}
}
