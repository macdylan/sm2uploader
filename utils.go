package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func searchInDir(exp string, dirpath string) (abspath string, err error) {
	dirInfo, err := os.Lstat(dirpath)
	if err != nil {
		return abspath, err
	}

	// check if the dir is a symbolic link
	if dirInfo.Mode()&os.ModeSymlink != 0 {
		dirpath, err = os.Readlink(dirpath)
	}

	err = filepath.Walk(dirpath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.HasPrefix(info.Name(), exp) {
			abspath = path
			return nil
		}
		return nil
	})
	return abspath, err
}

func postprocessGcodeFile(cmd string, file string) error {
	p := exec.Command(cmd, file)
	if out, err := p.CombinedOutput(); err != nil {
		// ignore
		if bytes.Contains(out, []byte(`No need to fix again`)) {
			return nil
		}
		return err
	}
	return nil
}
