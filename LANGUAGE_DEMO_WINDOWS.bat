@echo off
setlocal
cd /d "%~dp0"
if exist "language-demo.exe" (
  "language-demo.exe"
) else (
  go run ./cmd/language-demo
)
echo.
pause
