@echo off
setlocal
cd /d "%~dp0"
echo Student Records will be available at http://127.0.0.1:8081
echo On the first run, save the generated Admin password printed below.
echo.
if exist "student-records.exe" (
  "student-records.exe" -addr 127.0.0.1:8081 -db data\registry.db
) else (
  where go >nul 2>nul
  if errorlevel 1 (
    echo Install Go, then open this launcher again.
    pause
    exit /b 1
  )
  go run . -addr 127.0.0.1:8081 -db data\registry.db
)
echo.
pause
