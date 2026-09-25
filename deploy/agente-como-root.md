# El agente de `musubi-server` como root

Decisión del dueño, 2026-09-24: **todo `exec` y toda `shell` concedidos sobre `musubi-server` corren
como root, sin límites del lado del agente** (no hay allowlist nueva), y el token del agente se muda a
`/etc/musubi-agente/token`, fuera del alcance del usuario `musubi`.

Este archivo es la receta del despliegue, la vuelta atrás y los riesgos que se aceptaron. **Todo lo que
lleva `sudo` lo corre el dueño.** Nada de esto se aplica solo al instalar el binario.

## Qué trae el binario (y por qué va ANTES que el drop-in)

Un agente root con el binario de antes enumera el mundo de ROOT: `podman ps` lee el store rootful
(`/var/lib/containers`, vacío) y el cerebro —que poda por ausencia— da de **baja los 18 contenedores**
del dueño en el primer latido. El binario nuevo trae:

- **`MUSUBI_AGENTE_USUARIO`**: con el agente como root, nombra al usuario cuyo mundo se enumera. Podman
  y `systemctl --user` corren con `SysProcAttr.Credential` (uid, gid y grupos de ese usuario) y con su
  entorno (`HOME`, `USER`, `LOGNAME`, `XDG_RUNTIME_DIR`); sin binario externo ni PAM. El exec, la shell y
  el `systemctl` del sistema **siguen como root**. Un usuario inexistente, o `root`, hace que el agente
  **no arranque** (salir es mejor que enumerar el mundo equivocado).
- **Las units `--user` del dueño en el inventario** (`usuario:<unit>`): las escritas a mano en
  `~/.config/systemd/user/` y los quadlets de `~/.config/containers/systemd/` (Vaultwarden). Las de
  `/usr/lib` quedan afuera.
- **Techo de 96 servicios por latido** (era 64): con las units del dueño y los oneshots que entran al
  fallar, `musubi-server` llega a 73. Lo usan el agente **y el cerebro**: el cerebro va primero.
- **La rotación del token conserva el dueño del archivo** si el agente es root y el archivo era de otro.
- **`musubi agent --revisar-blindaje`** mira el directorio de `MUSUBI_DEVICE_TOKEN_FILE` y el home/runtime
  de la identidad, no el `$HOME` del proceso.

## Paso a paso

### 0 · Precondición: el binario nuevo, en el cerebro y en el agente

Compilado desde `main` con los build tags completos (los de `.github/workflows/release.yml`) e
instalado con la receta de siempre del cerebro. En este servidor `musubi-brain` y `musubi-agente`
comparten `/usr/local/bin/musubi` y se reinician juntos (`deploy/redesplegar-cerebro.sh`); el agente
lleva `After=musubi-brain.service`, así que su primer latido ya lo recibe el cerebro con el techo de
96. **Los agentes de las otras máquinas (davantis-1, gio) se actualizan DESPUÉS del cerebro**: uno
nuevo que mande más de 64 servicios a un cerebro viejo ve su inventario entero descartado.

```bash
/usr/local/bin/musubi agent --help | grep -c MUSUBI_AGENTE_USUARIO     # -> 1 (un binario viejo da 0)
```

**Con este paso el agente todavía corre como `musubi`**, y ya empieza a mandar las `usuario:*`: salta
`ServicioCaido` por `usuario:core01-ensayo-local`, que hoy está FAILED y nadie veía. Es lo buscado,
pero avisalo antes. En el journal tiene que aparecer `inventario de usuario: musubi (uid 1000)`.

Antes de instalar conviene probar la fuente `--user` **dentro del sandbox real** (pide OK: es un exec
en el server): `musubi_fleet_exec device=musubi-server` con argv
`[env, XDG_RUNTIME_DIR=/run/user/1000, systemctl, --user, show, *.service, --property=Id,FragmentPath,SourcePath,UnitFileState]`.
Si devuelve bloques, la fuente anda bajo el blindaje; si falla, el inventario se abortaría hasta el paso 3.

### 1 · Foto previa (sin sudo)

- `musubi_fleet_services device=musubi-server`: anotá cuántas filas `podman` hay (hoy 18) y sus nombres.
- `stat -c '%U:%G %a' /home/musubi/.config/musubi-agente /home/musubi/.config/musubi-agente/token`
- `find /home/musubi/.config/musubi-agente -user root | wc -l` → `0`

### 2 · El token a /etc (sudo)

```bash
sudo install -d -m 700 -o root -g root /etc/musubi-agente
sudo install -m 600 -o root -g root /home/musubi/.config/musubi-agente/token /etc/musubi-agente/token
sudo restorecon -Rv /etc/musubi-agente
```

El archivo viejo **no se borra todavía**: es la vuelta atrás.

### 3 · El drop-in (sudo)

**El nombre importa.** Ya existen `contenedores.conf` y `token-por-archivo.conf`, y systemd aplica los
drop-ins en orden alfabético: el último gana. `root.conf` ordena ANTES que `token-por-archivo.conf`
(`r` < `t`), así que su `MUSUBI_DEVICE_TOKEN_FILE` volvería a apuntar al home. Por eso **`zz-root.conf`**.

`/etc/systemd/system/musubi-agente.service.d/zz-root.conf`:

```ini
# El agente de musubi-server corre como root (decisión del dueño, 2026-09-24).
# Ver deploy/agente-como-root.md en el repo. Vuelta atrás: borrar este archivo y reponer el token.
[Service]
User=root
Group=root
SupplementaryGroups=
NoNewPrivileges=no
ProtectSystem=no
ProtectHome=no
PrivateTmp=no
ReadWritePaths=
Environment=HOME=/root
Environment=SHELL=/bin/bash
Environment=MUSUBI_AGENTE_USUARIO=musubi
Environment=MUSUBI_DEVICE_TOKEN_FILE=/etc/musubi-agente/token
```

`ReadWritePaths=` y `SupplementaryGroups=` vacíos **resetean** lo que traen la unidad base y
`contenedores.conf`; sin blindaje de montaje ya no hacen falta. El `After=user@1000.service` de
`contenedores.conf` se queda: es lo que hace que el bus del dueño exista cuando el agente arranca.

```bash
sudo restorecon -v /etc/systemd/system/musubi-agente.service.d/zz-root.conf
sudo systemctl daemon-reload
systemctl show musubi-agente -p DropInPaths -p User -p Environment   # zz-root.conf ÚLTIMO; User=root;
                                                                     # MUSUBI_DEVICE_TOKEN_FILE=/etc/...
sudo systemctl restart musubi-agente
```

(`systemctl show -p Environment` no imprime el token: la unidad lo lee de archivo. No uses
`systemctl status`, que imprime la línea de `ExecStart` entera.)

### 4 · Verificación, con números

| Qué | Cómo | Esperado |
|---|---|---|
| corre como root | `grep -E '^(Uid\|NoNewPrivs)' /proc/$(systemctl show -p MainPID --value musubi-agente)/status` | `Uid: 0 0 0 0` · `NoNewPrivs: 0` |
| enumera como el dueño | `journalctl -u musubi-agente --since -5min \| grep -E 'inventario de usuario\|latido registrado\|no se pudieron enumerar'` | `inventario de usuario: musubi (uid 1000)` y latidos, **sin** «no se pudieron enumerar» |
| los contenedores siguen | `musubi_fleet_services device=musubi-server` a los ~2 min | las mismas 18 `podman` con los mismos nombres, `fresco: true`, `ultimo_reporte` posterior al reinicio, más las `usuario:*` |
| el exec es root | `musubi_fleet_exec` `id -u` · `printenv HOME SHELL USER` | `0` · `/root` `/bin/bash` `root` |
| la carpeta vieja del token | `find /home/musubi/.config/musubi-agente -user root \| wc -l` | `0` |

Si las `podman` bajan a 0 o cambian de nombre: **vuelta atrás ya**, antes de que la poda se consolide.

## Vuelta atrás (sudo)

```bash
sudo rm /etc/systemd/system/musubi-agente.service.d/zz-root.conf
# Si el token rotó mientras corría como root, el vigente está en /etc: se lo devuelve a musubi.
sudo install -m 600 -o musubi -g musubi /etc/musubi-agente/token /home/musubi/.config/musubi-agente/token
sudo systemctl daemon-reload
sudo systemctl restart musubi-agente
```

Comprobar: `Uid: 1000` en `/proc/<MainPID>/status`, las 18 `podman` en `musubi_fleet_services` y el
journal con `inventario de usuario: musubi (uid 1000)`. El binario nuevo corriendo como `musubi` es un
estado válido (es el del paso 0): la vuelta atrás no exige reinstalar.

## Riesgos que quedan (aceptados por el dueño, no se mitigan en este PR)

- **uid `musubi` ≈ root en `musubi-server`.** El agente toma de la cola lo que encuentra pendiente sin
  volver a autorizarlo (`tomarComandosEnTx`, `internal/memory/comandos.go`), y `memory.db` es
  `musubi:musubi 0644`. Cualquier proceso que corra como `musubi` —el cerebro, el gateway, los
  puentes— puede escribir una fila en `device_commands` y hacerla ejecutar como root.
- **`davantis-2` es una shell root que no vence.** Es la única credencial vigente con `exec` + `shell`
  `['*']` sobre el server, sin vencimiento ni allowlist. El dueño decidió no ponerle vencimiento todavía.
- **El entorno del exec y de la shell es el de root** (`HOME=/root`). Para ver el mundo de `musubi`
  desde un exec hay que anteponer
  `setpriv --reuid=musubi --regid=musubi --init-groups env HOME=/home/musubi XDG_RUNTIME_DIR=/run/user/1000 …`
  (por ejemplo, `systemctl --user restart <unit>` de un puente).
- **Con el token en /etc**, un agente que vuelva a correr como `musubi` sin reponer el archivo del home
  arranca con un token viejo si hubo rotación: por eso la vuelta atrás lo copia de vuelta.
