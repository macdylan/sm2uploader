[中文说明|Chinese Readme](README.zh-cn.md)

# sm2uploader
A command-line tool for send the gcode file to Snapmaker Printers via WiFi connection.

## Features:
- Support Snapmaker 2 A150/250/350, J1, Artisan
- Auto discover machines (UDP broadcast)
- No need to click Yes button on the touch screen every time for authorization connect
- Support for multiple platforms including win/macOS/Linux/RaspberryPi

## Usage:
Download [sm2uploader](https://github.com/macdylan/sm2uploader/releases)
  - Linux/macOS: `chmod +x sm2uploader`

```
$ sm2uploader /path/to/code-file1 /path/to/code-file2
Discovering ...
Use the arrow keys to navigate: ↓ ↑ → ←
? Found 2 machines:
  ▸ A350-3DP@192.168.1.20 - Snapmaker A350
    A250-CNC@192.168.1.18 - Snapmaker A250
    J1V19@192.168.1.19 - Snapmaker-J1
Printer IP: 192.168.1.20
Printer Model: Snapmaker J1
Uploading file 'code-file1' [1.2 MB]...
  - SACP sending 100%
Upload finished.
```

If UDP Discover can not work, use `sm2uploader -host 192.168.1.20 /file.gcode` to directly upload to printer.

If `host` in `knownhosts`, `-host printer-id` is very convenient.

Get help: `sm2uploader -h`
