@echo off
setlocal EnableDelayedExpansion

title Archive Duplicate Finder - Smart Launcher Pro

set "CONFIG_FILE=launcher_config.bat"
if exist "%CONFIG_FILE%" call "%CONFIG_FILE%"

if "%LAST_DIR%"=="" set "LAST_DIR=%CD%"
if "%LAST_WEB%"=="" set "LAST_WEB=Y"
if "%LAST_PORT%"=="" set "LAST_PORT=8080"
if "%LAST_MODE%"=="" set "LAST_MODE=all"
if "%LAST_SIMILAR%"=="" set "LAST_SIMILAR=N"
if "%LAST_DELETE%"=="" set "LAST_DELETE=none"
if "%LAST_TRASH%"=="" set "LAST_TRASH=./trash"
if "%LAST_REF%"=="" set "LAST_REF=N"
if "%LAST_PDF%"=="" set "LAST_PDF=N"
if "%LAST_THRESHOLD%"=="" set "LAST_THRESHOLD=70"
if "!LAST_RECURSIVE!"=="" set "LAST_RECURSIVE=Y"
if "!LAST_DEBUG!"=="" set "LAST_DEBUG=N"

cls
echo ========================================================
echo   ARCHIVE DUPLICATE FINDER - SMART LAUNCHER PRO
echo ========================================================
echo.

echo 1. Target Directory
echo    Default: [%LAST_DIR%]
set "INPUT_DIR="
set /p "INPUT_DIR=> "
if "!INPUT_DIR!"=="" set "INPUT_DIR=%LAST_DIR%"
echo    Selected: !INPUT_DIR!
echo.

echo 2. Analysis Mode (all / size / name)
echo    Default: [%LAST_MODE%]
set "INPUT_MODE="
set /p "INPUT_MODE=> "
if "!INPUT_MODE!"=="" set "INPUT_MODE=%LAST_MODE%"
echo    Selected: !INPUT_MODE!
echo.

echo 3. Recursive Scan? (Y/N)
echo    Default: [%LAST_RECURSIVE%]
set "INPUT_RECURSIVE="
set /p "INPUT_RECURSIVE=> "
if "!INPUT_RECURSIVE!"=="" set "INPUT_RECURSIVE=%LAST_RECURSIVE%"
set "INPUT_RECURSIVE=!INPUT_RECURSIVE:y=Y!"
set "INPUT_RECURSIVE=!INPUT_RECURSIVE:n=N!"
echo    Selected: !INPUT_RECURSIVE!
echo.

echo 4. Enable Web Dashboard? (Y/N)
echo    Default: [%LAST_WEB%]
set "INPUT_WEB="
set /p "INPUT_WEB=> "
if "!INPUT_WEB!"=="" set "INPUT_WEB=%LAST_WEB%"
set "INPUT_WEB=!INPUT_WEB:y=Y!"
set "INPUT_WEB=!INPUT_WEB:n=N!"
echo    Selected: !INPUT_WEB!
echo.

set "INPUT_PORT=%LAST_PORT%"
if "!INPUT_WEB!"=="Y" (
    echo 4a. Port
    echo     Default: [%LAST_PORT%]
    set "TEMP_PORT="
    set /p "TEMP_PORT=> "
    if not "!TEMP_PORT!"=="" set "INPUT_PORT=!TEMP_PORT!"
    echo     Selected: !INPUT_PORT!
    echo.
)

set "INPUT_SIMILAR=%LAST_SIMILAR%"
if not "!INPUT_MODE!"=="size" (
    echo 5. Run Similarity Analysis? (Y/N)
    echo    Default: [%LAST_SIMILAR%]
    set "TEMP_SIMILAR="
    set /p "TEMP_SIMILAR=> "
    if not "!TEMP_SIMILAR!"=="" set "INPUT_SIMILAR=!TEMP_SIMILAR!"
    set "INPUT_SIMILAR=!INPUT_SIMILAR:y=Y!"
    set "INPUT_SIMILAR=!INPUT_SIMILAR:n=N!"
    echo    Selected: !INPUT_SIMILAR!
    echo.

    set "INPUT_THRESHOLD=%LAST_THRESHOLD%"
    if "!INPUT_SIMILAR!"=="Y" (
        echo 5a. Threshold (0-100)
        echo     Default: [%LAST_THRESHOLD%]
        set "TEMP_THRESHOLD="
        set /p "TEMP_THRESHOLD=> "
        if not "!TEMP_THRESHOLD!"=="" set "INPUT_THRESHOLD=!TEMP_THRESHOLD!"
        echo     Selected: !INPUT_THRESHOLD!
        echo.
    )
)

echo 6. Delete Mode (none / oldest / contents)
echo    Default: [%LAST_DELETE%]
set "INPUT_DELETE="
set /p "INPUT_DELETE=> "
if "!INPUT_DELETE!"=="" set "INPUT_DELETE=%LAST_DELETE%"
echo    Selected: !INPUT_DELETE!
echo.

set "INPUT_TRASH=%LAST_TRASH%"
set "INPUT_REF=%LAST_REF%"
if /I "!INPUT_DELETE!"=="oldest" set "IS_DEL=Y"
if /I "!INPUT_DELETE!"=="contents" set "IS_DEL=Y"
if "!IS_DEL!"=="Y" (
    echo 6a. Trash Path
    echo     Default: [%LAST_TRASH%]
    set "TEMP_TRASH="
    set /p "TEMP_TRASH=> "
    if not "!TEMP_TRASH!"=="" set "INPUT_TRASH=!TEMP_TRASH!"
    
    echo 6b. Leave .txt Ref? (Y/N)
    echo     Default: [%LAST_REF%]
    set "TEMP_REF="
    set /p "TEMP_REF=> "
    if not "!TEMP_REF!"=="" set "INPUT_REF=!TEMP_REF!"
    set "INPUT_REF=!INPUT_REF:Y=Y!"
    set "INPUT_REF=!INPUT_REF:N=N!"
    set "INPUT_REF=!INPUT_REF:y=Y!"
    set "INPUT_REF=!INPUT_REF:n=N!"
    echo    Selected: !INPUT_REF!
    echo.
)

echo 7. Generate PDF? (Y/N)
echo    Default: [%LAST_PDF%]
set "INPUT_PDF="
set /p "INPUT_PDF=> "
if "!INPUT_PDF!"=="" set "INPUT_PDF=%LAST_PDF%"
set "INPUT_PDF=!INPUT_PDF:y=Y!"
set "INPUT_PDF=!INPUT_PDF:n=N!"
echo    Selected: !INPUT_PDF!
echo.

set "INPUT_PDF_FILE="
if "!INPUT_PDF!"=="Y" (
    for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value 2^>nul') do set "dt=%%I"
    if defined dt (
        set "YYYY=!dt:~0,4!"
        set "MM=!dt:~4,2!"
        set "DD=!dt:~6,2!"
        set "INPUT_PDF_FILE=Report_!YYYY!-!MM!-!DD!.pdf"
    ) else (
        set "INPUT_PDF_FILE=Report.pdf"
    )
)


echo 8. Enable Debug? (Y/N)
echo    Default: [%LAST_DEBUG%]
set "INPUT_DEBUG="
set /p "INPUT_DEBUG=> "
if "!INPUT_DEBUG!"=="" set "INPUT_DEBUG=%LAST_DEBUG%"
set "INPUT_DEBUG=!INPUT_DEBUG:y=Y!"
set "INPUT_DEBUG=!INPUT_DEBUG:n=N!"
echo    Selected: !INPUT_DEBUG!
echo.

(
    echo set "LAST_DIR=!INPUT_DIR!"
    echo set "LAST_MODE=!INPUT_MODE!"
    echo set "LAST_RECURSIVE=!INPUT_RECURSIVE!"
    echo set "LAST_WEB=!INPUT_WEB!"
    echo set "LAST_PORT=!INPUT_PORT!"
    echo set "LAST_SIMILAR=!INPUT_SIMILAR!"
    echo set "LAST_THRESHOLD=!INPUT_THRESHOLD!"
    echo set "LAST_DELETE=!INPUT_DELETE!"
    echo set "LAST_TRASH=!INPUT_TRASH!"
    echo set "LAST_REF=!INPUT_REF!"
    echo set "LAST_PDF=!INPUT_PDF!"
    echo set "LAST_DEBUG=!INPUT_DEBUG!"
) > "%CONFIG_FILE%"

if "!INPUT_DIR:~-1!"=="\" set "INPUT_DIR=!INPUT_DIR:~0,-1!"
if "!INPUT_DIR:~-1!"=="/" set "INPUT_DIR=!INPUT_DIR:~0,-1!"
if "!INPUT_TRASH:~-1!"=="\" set "INPUT_TRASH=!INPUT_TRASH:~0,-1!"
if "!INPUT_TRASH:~-1!"=="/" set "INPUT_TRASH=!INPUT_TRASH:~0,-1!"

set "EXE_PATH=%CD%\archive-finder.exe"
if not exist "!EXE_PATH!" (
    echo ERROR: archive-finder.exe NOT FOUND
    pause
    exit /b 1
)

set "ARGS=-dir "!INPUT_DIR!" -mode !INPUT_MODE!"
if "!INPUT_RECURSIVE!"=="N" (
    set "ARGS=!ARGS! -recursive=false"
)
if "!INPUT_WEB!"=="Y" (
    set "ARGS=!ARGS! -web -port !INPUT_PORT!"
)
if "!INPUT_SIMILAR!"=="Y" (
    set "ARGS=!ARGS! -check-similar -threshold !INPUT_THRESHOLD!"
)
if "!IS_DEL!"=="Y" (
    set "ARGS=!ARGS! -delete !INPUT_DELETE! -trash "!INPUT_TRASH!" -yes"
    if "!INPUT_REF!"=="Y" (
        set "ARGS=!ARGS! -ref"
    )
)

if "!INPUT_PDF!"=="Y" (
    set "ARGS=!ARGS! -pdf "!INPUT_PDF_FILE!""
)
if "!INPUT_DEBUG!"=="Y" (
    set "ARGS=!ARGS! -debug -verbose"
)

cls
echo LAUNCHING: archive-finder.exe !ARGS!
echo.
"!EXE_PATH!" !ARGS!
pause
