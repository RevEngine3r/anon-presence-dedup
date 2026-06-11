@echo off
REM build.bat — builds Go backend (linux/windows amd64) and React frontend dist

set ROOT=%~dp0
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

echo =^> Tidying Go modules...
go mod tidy
if %ERRORLEVEL% NEQ 0 goto :fail

echo =^> Building backend for Linux amd64...
cmd /c "set CGO_ENABLED=0&& set GOOS=linux&& set GOARCH=amd64&& go build -trimpath -ldflags "-s -w" -o "%OUT%\linux\server" .\cmd\server"
if %ERRORLEVEL% NEQ 0 goto :fail

echo =^> Building backend for Windows amd64...
cmd /c "set CGO_ENABLED=0&& set GOOS=windows&& set GOARCH=amd64&& go build -trimpath -ldflags "-s -w" -o "%OUT%\windows\server.exe" .\cmd\server"
if %ERRORLEVEL% NEQ 0 goto :fail

REM ---------------------------------------------------------------------------
REM React frontend
REM ---------------------------------------------------------------------------
cd /d "%FRONTEND%"

echo =^> Installing Node dependencies...
call npm install --no-audit --no-fund
if %ERRORLEVEL% NEQ 0 goto :fail

echo =^> Building frontend...
call npm run build
if %ERRORLEVEL% NEQ 0 goto :fail

REM ---------------------------------------------------------------------------
REM Copy artefacts
REM ---------------------------------------------------------------------------
echo =^> Copying frontend dist...
xcopy /e /i /q /y "%FRONTEND%\dist\*" "%OUT%\frontend\"
if %ERRORLEVEL% NEQ 0 goto :fail

echo =^> Copying config templates...
copy /y "%ROOT%\server.yml" "%OUT%\linux\server.yml" >nul
copy /y "%ROOT%\server.yml" "%OUT%\windows\server.yml" >nul

echo.
echo Build complete. Output:
echo   %OUT%\linux\server
echo   %OUT%\linux\server.yml
echo   %OUT%\windows\server.exe
echo   %OUT%\windows\server.yml
echo   %OUT%\frontend\
goto :eof

:fail
echo.
echo [ERROR] Build failed at the step above (exit code %ERRORLEVEL%)
exit /b 1
