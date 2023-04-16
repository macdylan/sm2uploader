[English Readme](README.md)

# sm2uploader
Luban 和 Cura with SnapmakerPlugin 对于新手很友好，但是我的大部分配置文件在 PrusaSlicer 中，切片后再使用 Luban 上传到打印机是非常低效的。
这个工具提供了一步上传的能力，你可以通过命令行一次上传多个 gcode/cnc/bin固件 等文件。

## 功能
- 支持 Snapmaker 2 A/J1/Artisan 全系列打印机
- 自动发现局域网内所有的 Snapmaker 打印机（和 Luban 相同的协议，使用 UDP 广播）
- Snapmaker 2 A-Series 第一次连接时需要授权，之后可以直接一步上传
- 支持 macOS/Windows/Linux/RaspberryPi 多个平台

## 使用方法
下载适用的[程序文件](https://github.com/macdylan/sm2uploader/releases)
  - Linux/macOS 下，可能需要赋予可执行权限 `chmod +x sm2uploader`

```
$ sm2uploader /path/to/code-file1 /path/to/code-file2
Discovering ...
Use the arrow keys to navigate: ↓ ↑ → ←
? Found 2 machines:
  ▸ A350-3DP@192.168.1.20 - Snapmaker 2 Model A350
    A250-CNC@192.168.1.18 - Snapmaker 2 Model A250
    J1V19@192.168.1.19 - Snapmaker J1
Printer IP: 192.168.1.20
Printer Model: Snapmaker J1
Uploading file 'code-file1' [1.2 MB]...
  - SACP sending 100%
Upload finished.
```

打印机的 UDP 应答服务有时会挂掉，通常需要重启打印机来解决。或者你可以直接指定目标IP: `sm2uploader -host 192.168.1.20 /file.gcode`

如果 `host` 被发现过或者连接过，它会存在于 `knownhosts` 中，直接使用 id 进行连接会更加简洁: `sm2uploader -host A350-3DP /file.gcode`

更多参数：`sm2uploader -h`
