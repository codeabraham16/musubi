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
Guarda el token, escribe el `.mcp.json`, y verifica. **El split-tunnel de NordVPN es GUI
(sin CLI):** si la verificación falla, el script imprime los clics exactos (protocolo
OpenVPN + agregar `tailscaled.exe`, `curl.exe`, `node.exe` a "Disable VPN for selected
apps"), y recordá el orden: **Tailscale conectado primero, NordVPN después.**

## Notas

- El token va **por referencia** (`${MUSUBI_TOKEN}`) en el `.mcp.json`: el secreto nunca
  toca el archivo (patrón de Musubi).
- Cada proyecto queda con su memoria **local aislada** + la entrada **remota** al cerebro
  compartido (dos entradas en el `.mcp.json`).
- Re-ejecutar `install-musubi-brain.sh` NO regenera el token (no rompe a los clientes).
- Usar SIEMPRE la **IP del tailnet** (no nombres MagicDNS): con NordVPN activo el DNS no
  resuelve los nombres de la malla.
