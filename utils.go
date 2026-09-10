package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	var buf bytes.Buffer
	if err = postProcessTo(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// postProcessTo parses the g-code from r, applies the SMFix modifiers and
// streams the result to w. The output is written incrementally so the
// processed file is never fully buffered in memory.
// NOTE: the parse stage still holds one GcodeBlock per line in memory
// (required by the whole-file-context modifiers). Measured costs are
// documented in AGENTS.md.
func postProcessTo(w io.Writer, r io.Reader) (err error) {
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
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	if !isFixed {
		funcs := []fix.GcodeModifier{}

		if fixShutoff {
			funcs = append(funcs, fix.GcodeFixShutoff)
		}
		if fixPreheat {
			funcs = append(funcs, fix.GcodeFixPreheat)
		}
		if fixReplaceTool {
			funcs = append(funcs, fix.GcodeReplaceToolNum)
		}
		// if fixReinforceTower {
		// 	funcs = append(funcs, fix.GcodeReinforceTower)
		// }

		funcs = append(funcs, fix.GcodeFixOrcaToolUnload)

		for _, fn := range funcs {
			gcodes = fn(gcodes)
		}

		if headers, err = fix.ExtractHeader(gcodes); err != nil {
			return err
		}
	}

	bw := bufio.NewWriter(w)
	for _, h := range headers {
		bw.Write(h)
		bw.Write(nl)
	}

	for _, gcode := range gcodes {
		bw.WriteString(gcode.String())
		bw.Write(nl)
	}
	return bw.Flush()
}

// preprocessToOutputDir streams the original content to
// <OutputDir>/<name> (optional) and the post-processed content to
// <OutputDir>/<base>_fixed<ext>, reading the input only once and without
// ever holding the whole file in memory. It returns the path of the fixed
// file so the upload can stream from disk.
func preprocessToOutputDir(r io.Reader, name string, saveOriginal bool) (fixedPath string, err error) {
	if OutputDir == "" {
		return "", nil
	}
	if err := os.MkdirAll(OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	fixedPath = filepath.Join(OutputDir, base+"_fixed"+ext)

	fixed, err := os.Create(fixedPath)
	if err != nil {
		return "", fmt.Errorf("failed to save fixed file: %w", err)
	}
	defer fixed.Close()

	var src io.Reader = r
	if saveOriginal {
		origPath := filepath.Join(OutputDir, name)
		orig, err := os.Create(origPath)
		if err != nil {
			os.Remove(fixedPath)
			return "", fmt.Errorf("failed to save original file: %w", err)
		}
		defer orig.Close()
		src = io.TeeReader(r, orig)
	}

	if err := postProcessTo(fixed, src); err != nil {
		os.Remove(fixedPath)
		return "", err
	}
	return fixedPath, nil
}

func shouldBeFix(fpath string) bool {
	ext := strings.ToLower(filepath.Ext(fpath))
	return SmFixExtensions[ext]
}

func parseIntEnv(key string, defaultValue int) int {
	if value, ok := os.LookupEnv(key); ok {
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	}
	return defaultValue
}

func parseBoolEnv(key string, defaultValue bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if v, err := strconv.ParseBool(value); err == nil {
			return v
		}
	}
	return defaultValue
}

func parseDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if v, err := time.ParseDuration(value); err == nil {
			return v
		}
	}
	return defaultValue
}
