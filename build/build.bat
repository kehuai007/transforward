@echo off
setlocal

set VERSION=1.0.0
set DIST=dist
mkdir %DIST% 2>nul

echo Building TransForward...

echo Building for Windows...
go build -ldflags="-s -w -X main.version=%VERSION%" -o %DIST%\transforward-windows-amd64.exe .

echo.
echo Build complete! Output in %DIST%:
dir %DIST%

endlocal
