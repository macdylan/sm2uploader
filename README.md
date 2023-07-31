[中文说明|Chinese Readme](README.zh-cn.md)

# sm2uploader
A command-line tool for send the gcode file to Snapmaker Printers via WiFi connection.

## Features:
- Support Snapmaker 2 A150/250/350, J1, Artisan
- Auto discover machines (UDP broadcast)
- Simulated a OctoPrint server, so that it can be in any slicing software such as Cura/PrusaSlicer/SuperSlicer/OrcaSlicer send gcode to the printer
- Smart pre-heat for switch tools, shutoff nozzles that are no longer in use, and other optimization features for multi-extruders.
- No need to click Yes button on the touch screen every time for authorization connect
- Support for multiple platforms including win/macOS/Linux/RaspberryPi

## Usage:
Download [sm2uploader](https://github.com/macdylan/sm2uploader/releases)
  - Linux/macOS: `chmod +x sm2uploader`

```
## Discover mode
$ sm2uploader /path/to/code-file1 /path/to/code-file2
Discovering ...
Use the arrow keys to navigate: ↓ ↑ → ←
? Found 3 machines:
  ▸ A350-3DP@192.168.1.20 - Snapmaker A350
    A250-CNC@192.168.1.18 - Snapmaker A250
    J1V19@192.168.1.19 - Snapmaker-J1
Printer IP: 192.168.1.19
Printer Model: Snapmaker J1
Uploading file 'code-file1' [1.2 MB]...
  - SACP sending 100%
Upload finished.

## Directly mode
$ sm2uploader -host 192.168.1.19 /path/to/code-file1 /path/to/code-file2
Printer IP: 192.168.1.19
Printer Model: Snapmaker J1
Uploading file 'code-file1' [1.2 MB]...
  - SACP sending 100%
Upload finished.

## use printer id
$ sm2uploader -host J1V19 /path/to/code-file1 /path/to/code-file2
Discovering ...
Printer IP: 192.168.1.19
Printer Model: Snapmaker J1
Uploading file 'code-file1' [1.2 MB]...
  - SACP sending 100%
Upload finished.

## OctoPrint server (CTRL-C to stop)
$ sm2uploader -octoprint :8844 -host A350
Printer IP: 192.168.1.20
Printer Model: Snapmaker 2 Model A350
Starting OctoPrint server on :8844 ...
Server started, now you can upload files to http://localhost:8844
Request GET /api/version completed in 6.334µs
  - HTTP sending 100.0%
Upload finished: model.gcode [382.2 KB]
Request POST /api/files/local completed in 951.080458ms
...
```

If UDP Discover can not work, use `sm2uploader -host 192.168.1.20 /file.gcode` to directly upload to printer.

If `host` in `knownhosts`, `-host printer-id` is very convenient.

Get help: `sm2uploader -h`

## Fix the "can not be opened because it is from an unidentified developer"

Solution: https://osxdaily.com/2012/07/27/app-cant-be-opened-because-it-is-from-an-unidentified-developer/

or:
`xattr -d com.apple.quarantine sm2uploader-darwin`
