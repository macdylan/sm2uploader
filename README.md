# sm2uploader
a py script for send the gcode file to Snapmaker 2 via WiFi

## Features:
- Auto discover machines (UDP broadcast)
- No need to click Yes button on the touch screen every time for authorization connect (keep token in tempdir)
- Easy to use

## Usage:
```
$ sm2uploader.py /path/to/file
Discovering ...

Found 2 machines:
> Snapmaker-DUMY@127.0.0.1|model:Snapmaker 2 Model A350|status:IDLE [ip: 10.0.1.28]
> Snapmaker-A350@10.0.1.27|model:Snapmaker 2 Model A350|status:IDLE [ip: 10.0.1.27]

Use 'sm2uploader.py /path/to/file ip' to specify the target machine

$ sm2uploader.py /path/to/file
Discovering ...

IP Address	: 10.0.1.27
Token		: 406fa8be-3853-44eb-a8e7-210871733b21
Payload		: file
Payload size(b)	: 37517

Sending ... Success ✅
Start print this file on the touchscreen.

$ sm2uploader.py /path/to/file 10.0.1.27
IP Address	: 10.0.1.27
Token		: 406fa8be-3853-44eb-a8e7-210871733b21
Payload		: file
Payload size(b)	: 37517

Sending ... Success ✅
Start print this file on the touchscreen.
```
