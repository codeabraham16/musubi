package crudobackup

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// transporteSCP es el transporte real: scp por SSH, con la llegada confirmada por `ssh host
// sha256sum`. Es el único transporte validado (davantis-1 → musubi-server). Los binarios ssh/scp de
// OpenSSH —en Windows, los de C:\Windows\System32\OpenSSH\— están EXCLUIDOS del túnel de NordVPN (a
// diferencia de tailscaled); se invocan por nombre y los resuelve el PATH. La autenticación la pone
// el SSH de la máquina (claves en ~/.ssh): este código nunca ve una credencial.
type transporteSCP struct {
	waitSeconds int
}

func (t transporteSCP) Enviar(paquete, remote string) (string, error) {
	userHost, rutaRemota, err := partirRemote(remote)
	if err != nil {
		return "", err
	}
	if t.waitSeconds > 0 {
		if err := esperarRed(userHost, t.waitSeconds); err != nil {
			return "", err
		}
	}
	// La ruta remota es siempre unix; el basename del paquete sale del FS local. Se nombra el
	// archivo destino explícitamente (no el directorio), para no depender de barras finales.
	destino := path.Join(rutaRemota, filepath.Base(paquete))

	scp := exec.Command("scp", "-q", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", paquete, userHost+":"+destino)
	if out, err := scp.CombinedOutput(); err != nil {
		return "", fmt.Errorf("scp %s → %s: %v: %s", paquete, remote, err, strings.TrimSpace(string(out)))
	}

	// Confirmación de llegada: el sha lo calcula el DESTINO sobre el archivo recién escrito.
	ssh := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", userHost, "sha256sum", destino)
	out, err := ssh.Output()
	if err != nil {
		return "", fmt.Errorf("sha256sum remoto de %s: %w", destino, err)
	}
	campos := strings.Fields(string(out))
	if len(campos) == 0 {
		return "", fmt.Errorf("sha256sum remoto de %s: salida vacía", destino)
	}
	return campos[0], nil
}

// partirRemote separa "user@host:/ruta" en su parte de conexión y su ruta remota (unix). El primer
// ':' es el separador —vale para IPv4 y hostnames, el caso del tailnet—.
func partirRemote(remote string) (userHost, ruta string, err error) {
	remote = strings.TrimSpace(remote)
	i := strings.Index(remote, ":")
	if i <= 0 || i == len(remote)-1 {
		return "", "", fmt.Errorf("remote inválido %q: se espera user@host:/ruta", remote)
	}
	userHost = remote[:i]
	// Blindaje: userHost va como argumento POSICIONAL a scp/ssh. Si empezara con '-', esos binarios lo
	// tomarían como una opción (p.ej. -oProxyCommand=…) en vez de un destino —argument injection—. El
	// remote es config de confianza del dueño, así que es defensa en profundidad; el costo es una línea.
	if strings.HasPrefix(userHost, "-") {
		return "", "", fmt.Errorf("remote inválido %q: el host no puede empezar con '-' (scp/ssh lo tomarían como opción)", remote)
	}
	return userHost, remote[i+1:], nil
}

// esperarRed sondea el destino con un ssh liviano hasta que responde o se agota el plazo: una laptop
// que recién despierta puede no estar alcanzable en el primer intento.
func esperarRed(userHost string, segundos int) error {
	deadline := time.Now().Add(time.Duration(segundos) * time.Second)
	var ultimo error
	for {
		if err := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", userHost, "true").Run(); err == nil {
			return nil
		} else {
			ultimo = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("el destino %s no respondió en %ds: %v", userHost, segundos, ultimo)
		}
		time.Sleep(2 * time.Second)
	}
}
