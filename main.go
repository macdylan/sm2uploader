package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/manifoldco/promptui"
)

var (
	Host            string
	KnownHosts      string
	DiscoverTimeout time.Duration

	_Payloads []string
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			os.Exit(2)
		}
	}()

	// 获取程序所在目录
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Panicln(err)
	}
	defaultKnownHosts := filepath.Join(dir, "hosts.yaml")

	flag.StringVar(&Host, "host", "", "upload to host(id/ip/hostname), not required.")
	flag.StringVar(&KnownHosts, "knownhosts", defaultKnownHosts, "known hosts")
	flag.DurationVar(&DiscoverTimeout, "timeout", time.Second*4, "printer discovery timeout")

	flag.Usage = flag_usage
	flag.Parse()

	args := flag.Args()
	// prusaslicer
	if outputName := os.Getenv("SLIC3R_PP_OUTPUT_NAME"); outputName != "" {
		args = []string{outputName}
	}

	// 检查这些文件参数是否存在
	for _, file := range args {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			log.Panicf("File %s does not exist\n", file)
		} else {
			_Payloads = append(_Payloads, file)
		}
	}

	// 检查是否有传入的文件
	if len(_Payloads) == 0 {
		log.Panicln("No input files")
	}

	var printer *Printer
	ls := NewLocalStorage(KnownHosts)
	defer func() {
		if printer != nil {
			// update printer's token
			ls.Add(printer)
		}
		ls.Save()
	}()

	// Check if host is specified
	printer = ls.Find(Host)
	if printer != nil {
		log.Println("Found printer in " + KnownHosts)
	}

	// Discover printers
	if printer == nil {
		log.Println("Discovering ...")
		if printers, err := Discover(DiscoverTimeout); err == nil {
			ls.Add(printers...)
		}
		printer = ls.Find(Host)
		if printer != nil {
			log.Printf("Found printer: %s", printer.String())
		}
	}

	if printer == nil {
		if Host == "" {
			// Prompt user to select a printer
			printers := ls.Printers
			if len(printers) == 0 {
				log.Panicln("No printers found")
			}
			if len(printers) > 1 {
				prompt := promptui.Select{
					Label: "Select a printer",
					Items: printers,
				}
				idx, _, err := prompt.Run()
				if err != nil {
					log.Panicln(err)
				}
				printer = printers[idx]
			} else {
				printer = printers[0]
			}
		} else {
			// directly to printer using ip/hostname
			printer = &Printer{IP: Host}
		}
	}

	log.Println("Printer IP:", printer.IP)
	if printer.Model != "" {
		log.Println("Printer Model:", printer.Model)
	}

	// Upload files to host
	for _, filepath := range _Payloads {
		st, _ := os.Stat(filepath)
		fname := path.Base(filepath)
		log.Printf("Uploading file '%s' [%s]...", fname, humanReadableSize(st.Size()))
		if err := Connector.Upload(printer, filepath); err != nil {
			log.Panicln(err)
		} else {
			log.Println("Upload finished.")
			<-time.After(time.Second * 1)
		}
	}
}

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
