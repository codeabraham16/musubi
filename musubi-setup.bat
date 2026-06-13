@echo off
REM Musubi - inyeccion por doble clic.
REM Copia este .bat en la raiz de un proyecto y hace doble clic:
REM prepara el entorno de Musubi en esa carpeta y lo deja listo para Claude.
setlocal
cd /d "%~dp0"

where musubi >nul 2>nul
if %errorlevel%==0 (
    musubi setup
) else if exist "%~dp0musubi.exe" (
    "%~dp0musubi.exe" setup
) else (
    echo No se encontro 'musubi' en el PATH ni junto a este .bat.
    echo Instala Musubi ^(scripts/install.ps1^) o copia musubi.exe junto a este archivo.
)

echo.
pause
