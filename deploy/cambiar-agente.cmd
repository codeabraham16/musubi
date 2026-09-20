@echo off
REM cambiar-agente.cmd - reemplaza el binario del agente de flota, con vuelta atras automatica.
REM
REM Se ejecuta DESPEGADO del agente a proposito: el paso 2 mata al agente, y el agente es quien
REM esta corriendo este comando. Un script que se suicida a la mitad deja la maquina sin canal.
REM
REM La prueba se hace EN LA RUTA DEFINITIVA, no en un nombre temporal. En la maquina con NordVPN
REM el split tunneling autoriza por RUTA: musubi-nuevo.exe queda bloqueado (WSAEACCES) y
REM musubi.exe no. Probar con otro nombre daria un falso negativo garantizado.
REM
REM ASCII puro, sin acentos: PowerShell 5.1 y cmd.exe con UTF-8 sin BOM ya rompieron una vez.

REM LA CARPETA SALE DE DONDE ESTA ESTE ARCHIVO, no de %LOCALAPPDATA% (A71).
REM
REM Con -AlArranque el agente corre como SYSTEM, y ahi %LOCALAPPDATA% es
REM C:\WINDOWS\system32\config\systemprofile\AppData\Local -- no la instalacion. El script
REM no encontraba musubi-nuevo.exe, escribia NO HAY BINARIO NUEVO y salia sin tocar nada:
REM fallaba EN SILENCIO, que es justo lo que este script existe para no hacer.
REM %~dp0 es lo unico que no depende de QUIEN lo ejecuta.
set DIR=%~dp0
if %DIR:~-1%==\ set DIR=%DIR:~0,-1%
set TAREA=Musubi Agente de Flota
set LOG=%DIR%\cambio.log

echo === %DATE% %TIME% === > "%LOG%"

if not exist "%DIR%\musubi-nuevo.exe" (
  echo NO HAY BINARIO NUEVO, no se toca nada >> "%LOG%"
  exit /b 2
)

REM ESTA CARPETA TIENE QUE SER LA DEL AGENTE, Y SE COMPRUEBA ANTES DE TOCAR LA TAREA.
REM
REM %~dp0 dice desde donde corre este archivo, pero NO dice que sea el agente. Y el paso [1] para
REM la tarea POR NOMBRE GLOBAL ("Musubi Agente de Flota"), que es unica por maquina: o sea que una
REM copia de este .cmd en CUALQUIER carpeta puede parar al agente de verdad y despues cambiar un
REM binario que no es el suyo.
REM
REM Paso el 2026-09-05 en davantis-1. El conductor eligio la carpeta de la app de escritorio
REM (AppData\Local\Programs\musubi) en vez de la del agente (AppData\Local\Musubi), copio este
REM .cmd ahi y lo lanzo: paro la tarea del agente REAL y cambio el musubi.exe de la app. Lo unico
REM que evito el destrozo fue una casualidad -- esa carpeta no tiene device.token, asi que el
REM paso [4] no pudo probar el binario nuevo y el rollback lo devolvio. Una casualidad no es una
REM defensa: ahora es un candado.
REM
REM device.token es el discriminante porque es la credencial del dispositivo, el mismo archivo que
REM el paso [4] usa para probar. Donde no esta, no hay agente que actualizar.
if not exist "%DIR%\device.token" (
  echo NO ES LA CARPETA DEL AGENTE: falta device.token en %DIR% >> "%LOG%"
  echo No se toca la tarea ni ningun binario. >> "%LOG%"
  exit /b 5
)

echo [1] deteniendo la tarea >> "%LOG%"
schtasks /end /tn "%TAREA%" >> "%LOG%" 2>&1

REM Y MATANDO LO QUE LA TAREA NO SE LLEVA, POR RUTA EXACTA.
REM
REM `schtasks /end` termina la tarea, no necesariamente al proceso: el envoltorio oculto
REM lanza al agente como hijo y ese hijo sobrevive. Peor: el paso [2] RENOMBRA el binario
REM en uso, asi que el proceso viejo sigue vivo corriendo desde musubi.exe.viejo, latiendo
REM con la version anterior y tomando el archivo. Medido el 2026-09-02 en davantis-1: un
REM agente v0.106.0 zombi llevaba HORAS latiendo despues de una actualizacion exitosa, y
REM su archivo no se podia borrar (Access is denied), lo que disparaba el rollback.
REM
REM POR RUTA EXACTA Y NO POR NOMBRE DE IMAGEN: en estas maquinas tambien corre la app de
REM escritorio en AppData\Local\Programs\musubi\musubi.exe. Un `taskkill /IM musubi.exe`
REM la cerraria de un saque, y el usuario no entenderia por que.
REM EL CARET NO VA ADENTRO DE COMILLAS, Y ESTA LINEA LO APRENDIO A LOS GOLPES.
REM
REM cmd.exe consume el caret como escape SOLO fuera de comillas. Adentro de una cadena entre
REM comillas dobles el pipe ya es literal, asi que el caret no hace falta -- y peor: viaja
REM literal hasta PowerShell, que corta con: A positional parameter cannot be found that
REM accepts argument, y ahi imprime el caret. O sea que este paso NO CORRIA: el matador de
REM zombis estaba definido y desconectado desde el dia que se escribio, y el zombi que existe
REM para matar quedaba vivo latiendo con la imagen vieja.
REM
REM Medido el 2026-09-19 en davantis-1 con dos .cmd identicos salvo el caret:
REM   con caret --> PowerShell corta con el error de arriba y no mata a nadie
REM   sin caret --> Count : 0, o sea que corre y filtra
REM El cambio.log de esa misma maquina lo tenia escrito desde el 2026-09-11, justo arriba del
REM LISTO: agente actualizado que lo tapaba.
REM
REM Sabotaje que lo vuelve a romper: reponer el caret delante de cualquiera de los dos pipes.
REM Lo cubre TestElCambiadorNoEscapaConCaretDentroDeComillas.
powershell -NoProfile -Command "Get-Process | Where-Object { $_.Path -eq '%DIR%\musubi.exe' -or $_.Path -eq '%DIR%\musubi.exe.viejo' } | Stop-Process -Force -ErrorAction SilentlyContinue" >> "%LOG%" 2>&1
REM Un momento para que suelte el archivo. Windows deja RENOMBRAR un exe en uso, pero no
REM sobreescribirlo, y el proceso tarda en morir.
ping -n 6 127.0.0.1 >nul

echo [2] apartando el binario viejo >> "%LOG%"
if exist "%DIR%\musubi.exe.viejo" del /f /q "%DIR%\musubi.exe.viejo" >> "%LOG%" 2>&1
move /y "%DIR%\musubi.exe" "%DIR%\musubi.exe.viejo" >> "%LOG%" 2>&1
if errorlevel 1 (
  echo NO SE PUDO APARTAR EL VIEJO, se reinicia lo que habia >> "%LOG%"
  schtasks /run /tn "%TAREA%" >> "%LOG%" 2>&1
  exit /b 3
)

echo [3] poniendo el nuevo en su lugar >> "%LOG%"
move /y "%DIR%\musubi-nuevo.exe" "%DIR%\musubi.exe" >> "%LOG%" 2>&1
if errorlevel 1 goto rollback

echo [4] probando el nuevo EN SU RUTA DEFINITIVA >> "%LOG%"
REM La RUTA del token y no su contenido: el `set /p` de antes metia la credencial en el entorno
REM de este proceso, y ademas probaba un camino distinto del que usa el lanzador de la tarea.
REM EL ENTORNO SE DERIVA DEL LANZADOR, NO SE COPIA. Aca estaba la direccion del cerebro
REM escrita a mano, con su IP y su puerto, y era una copia de la que vive en `agente.cmd`, que
REM es el archivo que la Tarea Programada usa DE VERDAD. Dos copias de una direccion se
REM contradicen el dia que una cambia: el dia que el cerebro pase a HTTPS en otro puerto
REM (A129), esta prueba iba a seguir discando el viejo, fallar, y disparar el rollback acusando
REM al binario nuevo de no latir. El binario estaria sano y nadie miraria aca.
REM Leer las lineas `set MUSUBI_` del lanzador trae tambien MUSUBI_BRAIN_TLS_NAME y
REM MUSUBI_ALCANCE si estan, sin tener que enumerarlas aca.
set MUSUBI_BRAIN_URL=
for /f "usebackq delims=" %%L in (`findstr /b /c:"set MUSUBI_" "%DIR%agente.cmd"`) do %%L

REM Y SI NO SE PUDO LEER, ES ROJO Y NO UN DEFAULT. Un `agent --once` sin URL no prueba nada:
REM fallaria por falta de configuracion y el rollback culparia al binario. "No pude leer el
REM lanzador" y "el agente nuevo no late" son cosas distintas y tienen que decirse distinto.
if not defined MUSUBI_BRAIN_URL (
  echo FALLO: no pude leer MUSUBI_BRAIN_URL de "%DIR%agente.cmd" >> "%LOG%"
  echo        sin la direccion del cerebro esta prueba no mide nada, asi que no se acepta el nuevo >> "%LOG%"
  goto rollback
)
set MUSUBI_DEVICE_TOKEN_FILE=%DIR%\device.token
"%DIR%\musubi.exe" agent --once >> "%LOG%" 2>&1
if errorlevel 1 goto rollback

echo [5] el nuevo late. Arrancando la tarea >> "%LOG%"
schtasks /run /tn "%TAREA%" >> "%LOG%" 2>&1
echo LISTO: agente actualizado >> "%LOG%"
exit /b 0

:rollback
echo [!] EL NUEVO NO LATE. Volviendo al viejo >> "%LOG%"
move /y "%DIR%\musubi.exe" "%DIR%\musubi-nuevo.exe" >> "%LOG%" 2>&1
move /y "%DIR%\musubi.exe.viejo" "%DIR%\musubi.exe" >> "%LOG%" 2>&1
schtasks /run /tn "%TAREA%" >> "%LOG%" 2>&1
echo ROLLBACK HECHO: sigue el binario viejo >> "%LOG%"
exit /b 4
