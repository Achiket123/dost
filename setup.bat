@echo off
echo DOST - Developer's AI CLI Tool Setup
echo ====================================
echo.

REM Build the application
echo Building DOST...
go build -o dost.exe .
if %errorlevel% neq 0 (
    echo Error: Build failed!
    pause
    exit /b 1
)
echo ✓ Build successful!
echo.

REM Check if config exists in home directory
set HOME_CONFIG=%USERPROFILE%\.dost.yamlc
if exist "%HOME_CONFIG%" (
    echo ✓ Configuration file already exists at %HOME_CONFIG%
) else (
    echo Creating configuration file...
    copy .dost.example.yaml "%HOME_CONFIG%" >nul
    if %errorlevel% equ 0 (
        echo ✓ Configuration file created at %HOME_CONFIG%
        echo.
        echo IMPORTANT: Please edit %HOME_CONFIG% and add your Gemini API key!
        echo Get your API key from: https://makersuite.google.com/app/apikey
    ) else (
        echo ! Warning: Could not create configuration file
    )
)
echo.

REM Clean up cache if it exists
if exist ".dost\cache.txt" (
    echo Cleaning up cache...
    go run utils\cache_cleanup_utility.go
    echo ✓ Cache cleaned up
    echo.
)

echo Setup complete!
echo.
echo Next steps:
echo 1. Edit %HOME_CONFIG% and add your Gemini API key
echo 2. Run: dost.exe "help me create a new project"
echo.
pause
