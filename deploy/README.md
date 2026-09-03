# deploy/ — automatización del cerebro central Musubi

Scripts que reproducen, en **un comando por máquina**, el montaje manual del cerebro
central (Fase 1) y el onboarding de cada dispositivo cliente. Ver el runbook conceptual
en [`../docs/Server_Brain_Onboarding.md`](../docs/Server_Brain_Onboarding.md).

```
   Cliente (laptop/PC) ──┐                      ┌─ install-musubi-brain.sh
   Cliente (laptop/PC) ──┼── malla Tailscale ───┤   (musubi serve como systemd)
   connect-brain-*.sh  ──┘   (WireGuard, privada)└─ Servidor casero = "el cerebro"
```

## 1. En el servidor — montar el cerebro

Requisitos: Linux (RHEL/Rocky/Fedora o Debian/Ubuntu), el usuario del daemon ya creado
(`useradd -m musubi`), `curl` y `openssl`.

```bash
sudo ./install-musubi-brain.sh
```

Hace, idempotente: descarga el binario (verifica sha256), `restorecon` (SELinux),
inicializa el workspace, configura el bloque `service:`, **genera el token una sola vez**,
crea y arranca el servicio systemd, mete `tailscale0` en la zona `trusted` del firewall,
y verifica `/readyz`. Al final imprime el **token** para los clientes.

Variables opcionales: `BRAIN_USER`, `BRAIN_HOME`, `BRAIN_ADDR`, `MUSUBI_VERSION`.

> El cerebro escucha en `0.0.0.0:7717` pero el firewall solo lo deja alcanzable por la
> malla (`tailscale0` = trusted). Auth por bearer token; sin TLS porque el tailnet
> (WireGuard) ya cifra el transporte.

## 2. En cada dispositivo — conectarlo al cerebro

Necesitás el **token** que imprimió el paso 1.

### Linux
```bash
MUSUBI_TOKEN=<token> ./connect-brain-linux.sh /ruta/al/proyecto
```
Instala/une Tailscale, agrega el allowlist de NordVPN (`100.64.0.0/10`) si NordVPN está,
escribe la entrada remota `musubi-cerebro` en el `.mcp.json` del proyecto, exporta el
token en tu perfil, y verifica alcance + auth. **En Linux todo es automático.**

### Windows (PowerShell)
```powershell
$env:MUSUBI_TOKEN="<token>"; .\connect-brain-windows.ps1 -ProjectDir "C:\ruta\al\proyecto"
```
Hace todo **desde 0**, idempotente: se auto-eleva a admin, **instala Tailscale si falta**
(winget/MSI) y lo une a la malla (opcional `-TailscaleAuthKey` para no abrir el navegador),
aplica el **fix de firewall que destraba NordVPN** (reglas `TS-Allow-Tailnet-In/Out` que
permiten `100.64.0.0/10` y le ganan al filtro WFP de NordVPN), guarda el token, escribe el
`.mcp.json`, y **verifica con `node`** (el runtime real de Claude Code) que el cerebro
responde y autentica — no con `curl.exe`, que NordVPN no excluye de forma fiable y da
falsos negativos.

**Único paso manual que queda (GUI de NordVPN, sin CLI):** poner el protocolo en **OpenVPN
(UDP)** y agregar `tailscaled.exe` + `node.exe` a "Disable VPN for selected apps". Si la
verificación falla, el script imprime los clics exactos. Orden estable: **Tailscale
conectado primero, NordVPN después**; cada cambio en el split-tunnel reconecta NordVPN.

> El fix de firewall + el split-tunnel son complementarios: el firewall permite el rango a
> nivel de sistema, el split-tunnel saca a los procesos del túnel. Con ambos, la PC llega al
> cerebro con NordVPN activa (probado en `kernelos-pc`).

## 3. En el servidor — monitoreo y alertas (opcional)

El cerebro expone `/metrics` (Prometheus) con contadores ricos, pero *nada dispara* sobre
ellos hasta que un Prometheus los scrapea y evalúa las reglas de
[`musubi-alerts.yml`](musubi-alerts.yml) (7 alertas: caído, backup off-host stale, outbox
muerto, índice vectorial sin entrenar, rechazos de cuota/authz, tasa de error de tools).

Turnkey, en el mismo server que el cerebro:

```bash
sudo ./prometheus/install-musubi-prometheus.sh
```

Idempotente: descarga Prometheus (verifica sha256 contra el `sha256sums.txt` del release),
crea el usuario de sistema, escribe [`prometheus/prometheus.yml`](prometheus/prometheus.yml)
(scrape a `127.0.0.1:7717/metrics` con el bearer por `credentials_file` — el token nunca
toca la config), carga las reglas, **valida con `promtool` antes de arrancar**, y levanta el
servicio systemd. La UI queda en `127.0.0.1:9099` (loopback; exponela por la malla o túnel
SSH si la querés remota).

> Las alertas se **evalúan y se ven** en `http://127.0.0.1:9099/alerts`, pero para que
> **notifiquen** (email/Telegram/Slack) hay que sumar Alertmanager + un canal y descomentar
> el bloque `alerting:` en `prometheus.yml`. Qué hacer ante cada alerta: [`RUNBOOK.md`](RUNBOOK.md).

## Notas

- El token va **por referencia** (`${MUSUBI_TOKEN}`) en el `.mcp.json`: el secreto nunca
  toca el archivo (patrón de Musubi).
- Cada proyecto queda con su memoria **local aislada** + la entrada **remota** al cerebro
  compartido (dos entradas en el `.mcp.json`).
- Re-ejecutar `install-musubi-brain.sh` NO regenera el token (no rompe a los clientes).
- Usar SIEMPRE la **IP del tailnet** (no nombres MagicDNS): con NordVPN activo el DNS no
  resuelve los nombres de la malla.

### Al reemplazar un archivo de reglas: `cat >`, nunca `install` ni `cp`

Pasó el 2026-09-03 y el síntoma no nombra la causa. Se subieron los dos archivos de reglas con
`install -m 644`, con el dueño correcto y el modo correcto, y la recarga contestó **HTTP 500**:

```
loading groups failed  err="open /etc/prometheus/rules/musubi-alerts-flota.yml: permission denied"
```

**`permission denied` sobre un archivo `-rw-r--r-- musubi musubi` no es de permisos POSIX: es
SELinux.** El servidor los tiene puestos, y un archivo NUEVO nace con la etiqueta del directorio
del usuario (`user_home_t`) en lugar de la que el contenedor puede leer (`container_file_t`).
`install` y `cp` crean un archivo nuevo; `cat >` y `sed -i`… no: **`cat >` escribe DENTRO del
inodo que ya existe y hereda su etiqueta, `sed -i` lo reemplaza igual que `install`.**

Verlo:

```
ls -lZ /home/musubi/musubi-prometheus/rules/*.yml
```

Arreglarlo sin adivinar la etiqueta —copiándola de un archivo que sí funcionaba—:

```
chcon --reference=<un .bak que cargaba> <el archivo nuevo>
```

**Lo que salvó la situación fue que Prometheus falla seguro**: rechaza el juego entero y deja
corriendo el anterior («previous rule set restored»), así que el monitoreo no se cayó. Pero la
trampa queda armada: los archivos malos ya están en disco, y **un reinicio del contenedor no
tendría un juego anterior que restaurar**. Por eso se arregla en el momento, no después.

`verificar-despliegue.sh` lo detecta —lo reporta como «desplegado A MEDIAS»— pero no puede decir
que la causa es la etiqueta: eso está acá.
