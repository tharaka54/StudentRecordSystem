@echo off
setlocal
cd /d "%~dp0"
echo Student Records will be available at http://127.0.0.1:8081
echo Keep this window open while using the application.
echo.
if exist "student-records.exe" (
  "student-records.exe" -demo -addr 127.0.0.1:8081 -db data\student_records.db
) else (
  where go >nul 2>nul
  if errorlevel 1 (
    echo Install Go, then open this launcher again.
    pause
    exit /b 1
  )
  go run . -demo -addr 127.0.0.1:8081 -db data\student_records.db
)
echo.
pause
