@echo off
REM ============================================================
REM  Musubi - instalador por doble clic.
REM  Copia este archivo en la raiz del repo y hace doble clic.
REM  Te pregunta si queres instalar Musubi SOLO en este repo
REM  (local, no toca la PC) o GLOBAL en tu PC.
REM ============================================================
setlocal
cd /d "%~dp0"

echo ============================================
echo            Instalador de Musubi
echo ============================================
echo.
echo  Donde queres instalar Musubi?
echo.
echo    [L] Solo este repo  (local, NO toca la PC ni el PATH)
echo    [G] Global en la PC (PATH del usuario, sin admin)
echo    [Q] Cancelar
echo.

choice /c LGQ /n /m "Eleccion (L/G/Q): "
if errorlevel 3 goto :cancel
if errorlevel 2 goto :global
if errorlevel 1 goto :local

:local
set "MUSUBI_SCOPE=local"
goto :run
:global
set "MUSUBI_SCOPE=global"
goto :run

:run
set "MUSUBI_DIR=%CD%"
echo.
echo Ejecutando instalador (%MUSUBI_SCOPE%)...
powershell -NoProfile -ExecutionPolicy Bypass -Command "irm -useb https://raw.githubusercontent.com/codeabraham16/musubi/main/scripts/install.ps1 | iex"
goto :end

:cancel
echo Cancelado. No se instalo nada.

:end
echo.
pause
endlocal
