# S7b — la shell y el exec contra un `sshd` DE VERDAD (cierra A28)

## Cómo se pudo hacer sin instalar nada

A28 decía «no se pudo: esta máquina no tiene `openssh-server`, y levantar un servicio SSH es una
decisión del operador». Las dos mitades eran ciertas y la conclusión no: se puede **bajar el `.deb`
sin `sudo`, extraerlo, y correr un `sshd` SIN privilegios** en loopback.

```sh
apt-get download openssh-server            # no necesita sudo
dpkg-deb -x openssh-server_*.deb raiz
ssh-keygen -q -t ed25519 -f hostkey -N ''
cat > sshd.conf <<CFG
Port 2222
ListenAddress 127.0.0.1
HostKey  $PWD/hostkey
PidFile  $PWD/sshd.pid
AuthorizedKeysFile $PWD/authorized_keys
UsePAM no                 # PAM necesita root
StrictModes no            # las claves viven fuera de ~/.ssh
PasswordAuthentication no
AcceptEnv LINES COLUMNS
CFG
cp ~/.ssh/id_ed25519.pub authorized_keys   # la clave que YA existe: no se crea ninguna
raiz/usr/sbin/sshd -f sshd.conf -E sshd.log
```

Un `sshd` sin root sólo acepta al usuario que lo corre — que es exactamente el caso de prueba.
**Nada instalado, nada en systemd, nada persistente, nada más allá de loopback.**

Único rastro fuera del scratchpad: **una línea** en `~/.ssh/known_hosts`, respaldada y restaurada
al terminar (`diff` contra el original: idéntico).

### Y de paso, un techo que conviene tener escrito

OpenSSH resuelve `~` por **`getpwuid`, no por `$HOME`**. Correr el cerebro con un `HOME` de scratch
**no** aísla su `known_hosts` ni sus claves: `ssh` va igual al home real del usuario. El mensaje de
error de `explicarFalloSSH` ya dice lo correcto (`>> ~/.ssh/known_hosts`), y ahora se sabe por qué
no hay atajo.

## EL BUG QUE ESTABA ESPERANDO AHÍ: `--` de más

```go
args = append(args, "--", host, "--")   // ANTES
args = append(args, "--", host)         // AHORA
```

`ssh` **no interpreta** lo que va después del destino: lo junta con espacios y se lo entrega a la
shell de login del otro lado. Ese segundo `--` llegaba como parte del comando:

```
bash: --: invalid option
```

**Todos los exec de Tier B fallaban.** S7 nunca funcionó contra un `sshd` real, y ninguna prueba se
enteró: todas usan un `ssh` de mentira que registra argumentos y **nunca corre una shell**.

La razón declarada del segundo `--` era proteger un argv que empiece con guion. No hacía falta: el
citado ya lo da. Un `'-sr'` entre comillas simples llega como **nombre de comando**
(`-sr: command not found`), nunca como opción.

## La prueba que lo habría cachado, y su forma rara

`TestLoQueLlegaALaShellRemotaEsEjecutable` **corta los argumentos en el destino, junta el resto con
espacios y lo corre por una shell de verdad** — que es literalmente lo que hace el sshd. No
necesita un servidor: necesita dejar de simular la mitad que importa. 5 casos, 2 sabotajes
(el `--` de vuelta; sin citar).

El doble `sshFalso` quedó con su techo escrito encima, para que nadie vuelva a leerlo como
cobertura completa.

## Lo verificado contra el `sshd` real

| | |
|---|---|
| `-p` desde `user@host:2222` | ✅ el puerto sale del destino (S5c) |
| `StrictHostKeyChecking=yes` | ✅ **rechaza** sin la host key, con mensaje accionable; acepta con ella |
| autenticación por clave | ✅ `Accepted publickey ... ED25519` en el log del sshd |
| `exec` one-shot | ✅ `exit_code: 0`, salida real |
| citado | ✅ `$HOME` vuelve literal, sin expandir del otro lado |
| `-tt` y pty remoto | ✅ `tty` → `/dev/pts/2`, prompt interactivo con colores, bracketed paste |
| `SetEnv=LINES/COLUMNS` | ✅ llegan (`L=24 C=80`); **un solo `-o`**, separados por espacio |
| tamaño de terminal | ⚠️ `stty size` da **0 0** — ver abajo |
| cierre | ✅ `logout` → sesión `cerrada`, sin huérfanas ni zombies |

### El `0 0` de `stty size` no es un bug, es el precio de A27

El `ioctl` del pty remoto nace en cero porque el stdin del cerebro no es una terminal, así que ssh
manda un winsize 0×0 en el `pty-req`. Por eso `LINES`/`COLUMNS` viajan por entorno. Medido contra
el `sshd` real: **`tput lines`/`tput cols` devuelven 24 y 80, y `top` dibuja.** El fallback por
entorno funciona. Sigue siendo el motivo por el que A27 (SIGWINCH) no se puede hacer sin poseer el
pty maestro.

## Lo que queda fuera

- Un `sshd` con **PAM**, contraseñas, o `ForceCommand` — este corre sin root y sin PAM a propósito.
- Un host **remoto de verdad** (latencia, MTU, cortes): esto fue loopback.
- Otras implementaciones de servidor (dropbear, el `sshd` de un router, Windows OpenSSH).
