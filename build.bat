@echo off
REM build.bat — builds Go backend (linux/windows amd64) and React frontend dist
setlocal ENABLEDELAYEDEXPANSION

set ROOT=%~dp0
REM Strip trailing backslash from ROOT
if "%ROOT:~-1%"=="\" set ROOT=%ROOT:~0,-1%

set BACKEND=%ROOT%\backend
set FRONTEND=%ROOT%\frontend
set OUT=%ROOT%\dist

echo =^> Cleaning output directory...
if exist "%OUT%" rd /s /q "%OUT%"
mkdir "%OUT%\linux"
mkdir "%OUT%\windows"
mkdir "%OUT%\frontend"

REM ---------------------------------------------------------------------------
REM Go backend
REM ---------------------------------------------------------------------------
cd /d "%BACKEND%"

echo =^> Tidying Go modules (generates go.sum)...
go mod tidy
if ERRORLEVEL 1 ( echo [ERROR] go mod tidy failed & exit /b 1 )

echo =^> Building backend for Linux amd64...
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags "-s -w" -o "%OUT%\linux\server" .\cmd\server
if ERRORLEVEL 1 ( echo [ERROR] Linux build failed & exit /b 1 )

echo =^> Building backend for Windows amd64...
set GOOS=windows
go build -trimpath -ldflags "-s -w" -o "%OUT%\windows\server.exe" .\cmd\server
if ERRORLEVEL 1 ( echo [ERROR] Windows build failed & exit /b 1 )

set GOOS=
set GOARCH=
set CGO_ENABLED=

REM ---------------------------------------------------------------------------
REM React frontend
REM ---------------------------------------------------------------------------
cd /d "%FRONTEND%"

echo =^> Installing Node dependencies...
npm install
if ERRORLEVEL 1 ( echo [ERROR] npm install failed & exit /b 1 )

echo =^> Building frontend...
npm run build
if ERRORLEVEL 1 ( echo [ERROR] npm build failed & exit /b 1 )

REM ---------------------------------------------------------------------------
REM Copy artefacts
REM ---------------------------------------------------------------------------
echo =^> Copying frontend dist...
xcopy /e /i /y "%FRONTEND%\dist\*" "%OUT%\frontend\"
if ERRORLEVEL 1 ( echo [ERROR] xcopy frontend dist failed & exit /b 1 )

echo =^> Copying config templates...
copy "%ROOT%\server.yml" "%OUT%\linux\server.yml"
copy "%ROOT%\server.yml" "%OUT%\windows\server.yml"

echo.
echo Build complete. Output:
echo   %OUT%\linux\server          (Linux amd64 binary)
echo   %OUT%\linux\server.yml      (config template)
echo   %OUT%\windows\server.exe    (Windows amd64 binary)
echo   %OUT%\windows\server.yml    (config template)
echo   %OUT%\frontend\             (React static dist)

endlocal
