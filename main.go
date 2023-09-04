package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/manifoldco/promptui"
)

var (
	Host                string
	KnownHosts          string
	DiscoverTimeout     time.Duration
	OctoPrintListenAddr string
	NoFix               bool
	Debug               bool

	_Payloads       []*Payload
	SmFixExtensions = map[string]bool{
		".gcode": true,
		".nc":    false,
		".cnc":   false,
		".bin":   false,
	}
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			os.Exit(2)
		}
	}()

	Debug = os.Getenv("DEBUG") != ""

	// 获取程序所在目录
	ex, _ := os.Executable()
	dir, err := filepath.Abs(filepath.Dir(ex))
	if err != nil {
		log.Panicln(err)
	}
	defaultKnownHosts := filepath.Join(dir, "hosts.yaml")

	flag.StringVar(&Host, "host", "", "upload to host(id/ip/hostname), not required.")
	flag.StringVar(&KnownHosts, "knownhosts", defaultKnownHosts, "known hosts")
	flag.DurationVar(&DiscoverTimeout, "timeout", time.Second*4, "printer discovery timeout")
	flag.StringVar(&OctoPrintListenAddr, "octoprint", "", "octoprint listen address, e.g. '-octoprint :8844' then you can upload files to printer by http://localhost:8844")
	flag.BoolVar(&NoFix, "nofix", false, "disable SMFix(built-in)")
	flag.BoolVar(&Debug, "debug", false, "debug mode")

	flag.Usage = flag_usage
	flag.Parse()

	if Debug {
		log.Println("-- Debug mode")
	}

	if NoFix {
		log.Println("smfix disabled")
	}

	var printer *Printer
	ls := NewLocalStorage(KnownHosts)
	defer func() {
		if printer != nil {
			// update printer's token
			ls.Add(printer)
			if Debug {
				log.Printf("-- Updated printer: %s", printer.String())
			}
		}
		if err := ls.Save(); err == nil && Debug {
			log.Printf("-- Saved known hosts: %s", KnownHosts)
		}
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
			if Debug {
				log.Printf("-- Discovered %d printers", len(printers))
			}
			ls.Add(printers...)
		} else if Debug {
			log.Printf("-- Discover error: %s", err.Error())
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

	// Create a channel to listen for signals
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-sc
		log.Printf("Received signal: %s", sig)
		// update printer's token
		if printer != nil {
			ls.Add(printer)
			if Debug {
				log.Printf("-- Updated printer: %s", printer.String())
			}
		}
		if err := ls.Save(); err == nil && Debug {
			log.Printf("-- Saved known hosts: %s", KnownHosts)
		}
		os.Exit(0)
	}()

	if OctoPrintListenAddr != "" {
		// listen for octoprint uploads
		if err := startOctoPrintServer(OctoPrintListenAddr, printer); err != nil {
			log.Panic(err)
		}
		return
	}

	// 检查文件参数是否存在
	for _, file := range flag.Args() {
		if st, err := os.Stat(file); os.IsNotExist(err) {
			log.Panicf("File %s does not exist\n", file)
		} else {
			f, _ := os.Open(file)
			_Payloads = append(_Payloads, NewPayload(f, st.Name(), st.Size()))
		}
	}

	// 检查是否有传入的文件
	if len(_Payloads) == 0 {
		log.Panicln("No input files")
	}

	// 从 slic3r 环境变量中获取文件名
	envFilename := os.Getenv("SLIC3R_PP_OUTPUT_NAME")

	// Upload files to host
	for _, p := range _Payloads {
		if envFilename != "" {
			p.SetName(filepath.Base(envFilename))
		}

		log.Printf("Uploading file '%s' [%s]...", p.Name, p.ReadableSize())
		if err := Connector.Upload(printer, p); err != nil {
			log.Panicln(err)
		} else {
			log.Println("Upload finished.")
			<-time.After(time.Second * 1) // HMI needs some time to refresh
		}
	}
}
