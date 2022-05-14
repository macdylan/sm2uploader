# sm2uploader
A command-line tool for send the gcode file to Snapmaker 2 via WiFi connection.

## Features:
- Auto discover machines (UDP broadcast)
- No need to click Yes button on the touch screen every time for authorization connect
- Support for multiple platforms including win/macOS/Linux/RaspberryPi

## Usage:
Download [sm2uploader-{platform}-{arch}](https://github.com/macdylan/sm2uploader/releases/tag/go1.0)
  - Linux/macOS: `chmod +x sm2uploader`
  - Win: add prefix `.exe` that you can drag and drop files into icon to quick start

```
$ sm2uploader /path/to/code-file1 /path/to/code-file2
Discovering ...
Use the arrow keys to navigate: ↓ ↑ → ←
? Found 2 machines:
  ▸ Snapmaker-3DP-A350 <192.168.1.20>
    Snapmaker-CNC-A250 <192.168.1.18>

IP Address : 192.168.1.20
Token      : 2661c897-8b08-458b-a3b1-7b1b9063a61d
code-file1 100% |███████████████████████████████████████████████████████████████| (14/14 MB, 1.788 MB/s)
code-file2  84% |████████████████████████████████████████████████████           | (12/14 MB, 1.874 MB/s) [6s:1s]
```
