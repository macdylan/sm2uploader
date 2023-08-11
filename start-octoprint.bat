@echo off
echo Will start the sm2uploader in OctoPrint mode. press Ctrl+C to exit.

rem set host=-host A350
set host=

set /p port=Enter a local port num (default is 8899)
if "%port%"=="" set "port=8899"
if %port% LSS 1024 (
    echo Port number must be greater than 1024
    pause
    exit /b 1
)

set w64=sm2uploader-win64.exe
set w32=sm2uploader-win32.exe
set cmd=

where /q %w64% && set "cmd=%w64%"
where /q %w32% && set "cmd=%w32%"

if "%cmd%"=="" (
    echo Can not find %w64% or %w32%
    pause
    exit /b 1
)

echo %cmd% %host% -octoprint 127.0.0.1:%port%
%cmd% %host% -octoprint 127.0.0.1:%port%
pause
