package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

var (
	fIP      = flag.String("t", "", "target ip, e.g.: 192.168.1.200.")
	fID      = flag.String("n", "", "target name, e.g.: My\\ SnapmakerA350.")
	fStorage = flag.String("c", "", "a storage file to load and save tokens. (default sm2uploader.yaml)")
	fTimeout = flag.Uint("b", 2, "broadcast timeout, seconds.")

	// fForce if ture and fID, discover and find that ID.
	// only fID will load storage and find it, then discover if not found
	fForce = flag.Bool("r", false, "force to use discover.")

	// fPrint = flag.Bool("print", false, "upload and start printing the first file.")
)

var (
	_Conn     = NewConnector()
	_Storage  *LocalStorage
	_Payloads []*os.File
)

func loop() {
	waiting := false
	uploading := false
	uploaded := 0

	fire := make(chan bool, 1)
	stop := make(chan os.Signal, 1)

	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-stop:
			go func() {
				fmt.Println("Interrupting")
				if uploading {
					_Conn.Cancel()
				}
				_Conn.Disconnect()
				os.Exit(2)
			}()

		case <-fire:
			go func() {
				uploading = true
				defer func() { uploading = false }()

				pbar := mpb.New(mpb.WithRefreshRate(100 * time.Millisecond))
				bar := make([]*mpb.Bar, len(_Payloads))
				names := make([]string, len(_Payloads))
				for n, p := range _Payloads {
					st, _ := p.Stat()
					name := st.Name()
					if name == "stdin" {
						name = fmt.Sprintf("%d.gcode", time.Now().Unix())
					}
					bar[n] = pbar.AddBar(st.Size(),
						mpb.BarFillerClearOnComplete(),
						mpb.PrependDecorators(
							decor.Name(name),
							decor.CountersKibiByte("% .2f / % .2f", decor.WCSyncSpace),
						),
						mpb.AppendDecorators(
							decor.OnComplete(decor.Percentage(decor.WCSyncSpace), ""),
						),
					)
					names[n] = name
				}
				for n, p := range _Payloads {
					_Conn.Upload(names[n], bar[n].ProxyReader(p))
				}
				pbar.Wait()
			}()

		case state := <-_Conn.State:

			switch state {
			case ConnectorStateDiscovering:
				fmt.Print("Discovering ... ")

			case ConnectorStateDiscovered:
				if len(_Storage.Machines) == 0 {
					exit("Can not find any Snapmaker machine, please try again.\n" +
						"Or you can connect directly to the machine through -t IP.")
				}

				if len(_Storage.Machines) == 1 {
					target := _Storage.Machines[0]

					if *fID != "" && target.ID != *fID {
						exit("You specified -n '%s' but only one machine was found: %s<%s>",
							*fID, target.ID, target.IP)
					}

					fmt.Printf("%s <%s>\n", target.ID, target.IP)
					_Conn.Target = target

				} else {
					idx, _ := selectMachine(_Storage.Machines, *fID)
					_Conn.Target = _Storage.Machines[idx]
				}

				go _Conn.Connect()

			case ConnectorStateBroken:
				if uploading {
					_Conn.Cancel()
				}
				fmt.Println(_Conn.Error)
				return

			// case ConnectorStateConnecting:

			case ConnectorStateConnected:
				waiting = false
				if !uploading {
					fmt.Printf("IP Address : %s\n", _Conn.Target.IP)
					fmt.Printf("Token      : %s\n", _Conn.Target.Token)
					fire <- true
				}

			case ConnectorStatePrintingPrepared:
			case ConnectorStatePrinting:
			case ConnectorStatePrinted:
			case ConnectorStatePrintStopped:

			case ConnectorStateWaiting:
				if !waiting {
					waiting = true
					if _Conn.Target.ID != "" {
						fmt.Printf("%s <%s>\n", _Conn.Target.ID, _Conn.Target.IP)
					}
					fmt.Println("Please tap Yes on Snapmaker touchscreen to continue.")
				}

			case ConnectorStateUploaded:
				uploaded++
				if uploaded == len(_Payloads) {
					_Storage.SetLast(_Conn.Target)
					_Storage.Save()
					go _Conn.Disconnect()
				}

			case ConnectorStateDisconnected:
				// fmt.Println("disconnected")
				return
			}
		}
	}
}

func exit(msg string, args ...any) {
	fmt.Printf(msg, args...)
	fmt.Println("")
	os.Exit(1)
}

func prepare() {
	flag.Usage = usage
	flag.Parse()

	if len(flag.Args()) > 0 {
		for _, file := range flag.Args() {
			f, err := os.Open(file)
			checkFileError(f, err)
			_Payloads = append(_Payloads, f)
		}
	} else if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) == 0 {
		_Payloads = append(_Payloads, os.Stdin)
	}

	if len(_Payloads) == 0 {
		usage()
	}

	if *fStorage == "" {
		ex, _ := os.Executable()
		*fStorage = filepath.Join(filepath.Dir(ex), "sm2uploader.yaml")
	}
	_Storage = NewLocalStorage(*fStorage)

	var target *Machine

	if *fIP == "" && *fID == "" && !*fForce {
		if t := _Storage.GetLast(); t != nil && Probe(t.IP) {
			_Conn.Target = t
			return
		}
	}

	if *fIP != "" && Probe(*fIP) {
		if m := _Storage.FindByIP(*fIP); m != nil {
			target = m
		} else {
			target = &Machine{IP: *fIP}
		}
		_Conn.Target = target
		return
	}

	if *fID != "" && false == *fForce {
		if t := _Storage.FindByID(*fID); t != nil && Probe(t.IP) {
			_Conn.Target = t
			return
		}
	}

	if target == nil || *fForce {
		go func() {
			if *fForce {
				_Storage.Machines = []*Machine{} // reset
			}
			for m := range _Conn.Discover(time.Duration(*fTimeout)) {
				_Storage.Add(m)
			}
		}()
	}
}

func usage() {
	ex, _ := os.Executable()
	fmt.Printf("Usage:\n  %s [args] file1.gcode file2.gcode ...\n\n", filepath.Base(ex))
	fmt.Println("sm2uploader is a command-line tool for Snapmaker 2")
	fmt.Println("<https://github.com/macdylan/sm2uploader>")
	fmt.Println("\nArguments:")
	flag.PrintDefaults()
	os.Exit(1)
}

func selectMachine(machines []*Machine, id string) (idx int, err error) {
	items := []string{}
	for idx, item := range machines {
		items = append(items, fmt.Sprintf("%s <%s>", item.ID, item.IP))
		if id != "" && id == item.ID {
			return idx, nil
		}
	}

	prompt := promptui.Select{
		Label: fmt.Sprintf("Found %d machines", len(machines)),
		Items: items,
	}

	idx, _, err = prompt.Run()
	return
}

func checkFileError(fd *os.File, err error) {
	if err != nil {
		exit(err.Error())
	}
	if fd != nil {
		name := fd.Name()
		st, _ := fd.Stat()
		mode := st.Mode()

		if st.Size() <= 1 {
			exit("%s is empty.", name)
		}

		if st.IsDir() || (0 == (mode&os.ModeCharDevice) && !mode.IsRegular()) {
			exit("%s is not a regular file.", name)
		}
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Error: ", r)
		}
	}()

	prepare()

	if _Conn.Target != nil {
		go _Conn.Connect()
	}

	loop()
}
