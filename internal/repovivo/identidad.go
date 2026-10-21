package repovivo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// sanitizarRefComponent convierte un texto arbitrario (un hostname, el basename de un
// checkout) en un componente válido de un refname de git: sólo [A-Za-z0-9_.-], sin empezar
// ni terminar con '-' o '.', sin ".." y sin el sufijo ".lock" (todas reglas de git-check-ref-format).
// Nunca devuelve vacío.
func sanitizarRefComponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.ReplaceAll(b.String(), "..", "-")
	out = strings.TrimSuffix(out, ".lock")
	out = strings.Trim(out, "-.")
	if out == "" {
		return "x"
	}
	return out
}

// nodo es el nombre de esta máquina, sanitizado para un refname. Es la identidad POR MÁQUINA
// de la ref: dos máquinas distintas publican bajo refs/vivo/<nodo>/… distintos aunque trabajen
// el mismo checkout. Si el hostname no se puede leer, cae a un nombre fijo (no aborta).
func nodo() string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return "nodo-desconocido"
	}
	return sanitizarRefComponent(h)
}

// slugDeCheckout deriva un identificador estable y único POR CHECKOUT del path de su working
// tree (toplevel). Mismo checkout → mismo slug siempre; dos checkouts (incluidos dos worktrees
// del mismo repo, que tienen toplevels distintos) → slugs distintos. El basename legible va
// adelante para poder leer la ref, y un hash corto del path completo garantiza la unicidad aun
// cuando dos checkouts compartan basename en carpetas distintas.
func slugDeCheckout(toplevel string) string {
	limpio := filepath.Clean(toplevel)
	h := sha256.Sum256([]byte(limpio))
	base := sanitizarRefComponent(filepath.Base(limpio))
	if base == "x" {
		base = "repo"
	}
	return base + "-" + hex.EncodeToString(h[:])[:8]
}

// refDeCheckout arma la ref de la zona en vivo: refs/vivo/<nodo>/<slug>. Siempre bajo el
// prefijo que el transporte exige (fuera de refs/heads/, por el espejo de Altura).
func refDeCheckout(nodoNombre, slug string) string {
	return prefijoRefVivo + nodoNombre + "/" + slug
}

// hostDeURL extrae el host de una URL de remoto de forma LÉXICA y robusta, en minúsculas. Pela
// CUALQUIER esquema hasta "://" (así cubre también los compuestos git+ssh:// y ssh+git://), corta el
// userinfo en el ÚLTIMO '@' (OpenSSH y los navegadores usan el último, no el primero: ssh://a@b@h/
// resuelve a h), recorta en el primer '/' o ':' (ruta, o el puerto, o el ':' de la forma scp
// git@host:ruta) y quita un punto final de FQDN absoluto (github.com.). Es puro: no toca git, red ni ssh.
func hostDeURL(raw string) string {
	u := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.LastIndex(u, "@"); i >= 0 {
		u = u[i+1:]
	}
	if i := strings.IndexAny(u, "/:"); i >= 0 {
		u = u[:i]
	}
	return strings.TrimSuffix(u, ".")
}

// esHostGitHub informa si un host YA RESUELTO es github.com o un subdominio suyo. Un dominio
// corporativo que apenas contiene "github" (github.empresa.com) NO cuenta.
func esHostGitHub(host string) bool {
	return host == "github.com" || strings.HasSuffix(host, ".github.com")
}

// expandirInsteadOf devuelve la URL que git USARÍA para `destino`, con las reglas url.*.insteadOf
// aplicadas, SIN tocar la red: `git ls-remote --get-url` sólo imprime la URL expandida y sale. Así un
// repo_vivo.remote escrito como alias insteadOf (p.ej. "gh:acme/x" con url."git@github.com:".insteadOf
// "gh:") se resuelve a su destino real antes de juzgarlo. Si no se puede resolver, cae al destino crudo.
func expandirInsteadOf(ctx context.Context, git gitRunner, destino string) string {
	if rechazarFlags(destino) != nil {
		return destino
	}
	out, err := git.run(ctx, nil, "ls-remote", "--get-url", destino)
	if err != nil {
		return destino
	}
	if s := strings.TrimSpace(out); s != "" {
		return s
	}
	return destino
}

// salidaSSHMenosG corre `ssh -G <host>` —que SÓLO lee la configuración, NO conecta— y devuelve su
// salida cruda. Es una variable de paquete para que las pruebas inyecten un ~/.ssh/config aislado con
// `-F`: OpenSSH resuelve el config del usuario por el passwd (getpwuid), NO por $HOME, así que una
// prueba no puede aislarlo por entorno. En producción corre `ssh -G` a secas: el mismo config que
// git+ssh usaría al pushear.
var salidaSSHMenosG = func(ctx context.Context, host string) (string, error) {
	rutaSSH, err := exec.LookPath("ssh")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, rutaSSH, "-G", host)
	cmd.Env = entornoNoInteractivo(os.Environ())
	out, err := cmd.Output()
	return string(out), err
}

// hostnameDeSalidaSSH extrae el valor de la clave `hostname` de la salida de `ssh -G` (en minúsculas),
// que es el HostName real al que un alias se resuelve. Si no aparece, cae a `porDefecto`.
func hostnameDeSalidaSSH(out, porDefecto string) string {
	for _, l := range strings.Split(out, "\n") {
		if campos := strings.Fields(l); len(campos) == 2 && strings.ToLower(campos[0]) == "hostname" {
			return strings.ToLower(campos[1])
		}
	}
	return porDefecto
}

// resolverHostSSH resuelve un host que podría ser un ALIAS de ~/.ssh/config a su HostName real. Es lo
// que destapa el caso que mordió de verdad: un alias multi-cuenta "git@gh-trabajo:..." cuyo HostName es
// github.com. Fail-safe: si ssh no está en el PATH o la resolución falla, devuelve el host tal cual (cae
// al literal). rechazarFlags evita que un host que empiece con '-' se cuele como opción de ssh.
func resolverHostSSH(ctx context.Context, host string) string {
	if host == "" || rechazarFlags(host) != nil {
		return host
	}
	out, err := salidaSSHMenosG(ctx, host)
	if err != nil {
		return host
	}
	return hostnameDeSalidaSSH(out, host)
}

// destinoEsGitHub es la guarda DURA del espejo: decide si publicar por `destino` terminaría empujando
// a github.com RESOLVIENDO el destino real, en vez de mirar el string crudo (que una guarda léxica
// dejaba eludir con aliases, insteadOf, esquemas compuestos o userinfo con varios '@'). Orden: expandir
// insteadOf → extraer el host → resolver un alias ssh a su HostName → comparar. Todo sin red. El espejo
// público de Altura replica refs/heads/*+refs/tags/* a github.com, así que ante CUALQUIER destino que
// resuelva ahí la zona en vivo no arranca, ni con enabled=on.
func destinoEsGitHub(ctx context.Context, git gitRunner, destino string) bool {
	expandida := expandirInsteadOf(ctx, git, destino)
	host := resolverHostSSH(ctx, hostDeURL(expandida))
	return esHostGitHub(host)
}

// urlsPushDeRemoto devuelve TODAS las URLs de PUSH de un remoto y si ese remoto EXISTE en el
// checkout. Usa `get-url --push --all` a propósito: `git push <remoto>` va a la(s) pushurl si están
// configuradas, y a TODAS las URLs del remoto si no —y empuja a cada una—, así que mirar sólo la 1ª
// URL de fetch (lo que hacía la versión vieja) dejaba pasar un pushurl a github o una 2ª URL en
// github. Un remoto no declarado (o un nombre que parece flag) devuelve (nil, false), sin error.
func urlsPushDeRemoto(ctx context.Context, git gitRunner, remoto string) ([]string, bool) {
	if rechazarFlags(remoto) != nil {
		return nil, false
	}
	out, err := git.run(ctx, nil, "remote", "get-url", "--push", "--all", remoto)
	if err != nil {
		return nil, false
	}
	var urls []string
	for _, l := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(l); s != "" {
			urls = append(urls, s)
		}
	}
	return urls, len(urls) > 0
}

// remotoApuntaAGitHub decide si publicar por `remoto` terminaría empujando a github.com, y además
// informa si `remoto` es un remoto DECLARADO. Cubre los DOS caminos que `git push` puede tomar:
//   - `remoto` es un remoto declarado → mira TODAS sus URLs de push (una sola en github basta para
//     filtrar al espejo, así que apunta=true si cualquiera resuelve ahí); existe=true.
//   - `remoto` NO es un remoto declarado → puede ser una URL directa puesta en repo_vivo.remote, y
//     `git push <url>` empujaría ahí directo: se la evalúa tal cual; existe=false.
//
// Ante la duda (no se pudo leer y no es URL de github), apunta=false: encender lo decide debeEncender.
func remotoApuntaAGitHub(ctx context.Context, git gitRunner, remoto string) (apunta, existe bool) {
	urls, ok := urlsPushDeRemoto(ctx, git, remoto)
	if ok {
		for _, u := range urls {
			if destinoEsGitHub(ctx, git, u) {
				return true, true
			}
		}
		return false, true
	}
	return destinoEsGitHub(ctx, git, remoto), false
}

// debeEncender decide si la zona en vivo arranca y explica por qué (para el log de arranque):
//   - github.com es una guarda DURA previa a todo: si publicar por el remoto terminaría en github,
//     NO arranca ni con on. `apuntaAGitHub` ya resume ese juicio (pushurl, 2ª URL o URL directa).
//   - off → nunca; on → siempre (pasada la guarda dura).
//   - auto → sólo con team_mode activo Y un remoto empresarial declarado (que ya se sabe ≠ github).
func debeEncender(modo string, teamMode bool, apuntaAGitHub, remotoExiste bool) (bool, string) {
	if apuntaAGitHub {
		return false, "el remoto apunta a github.com (espejo público): la zona en vivo no arranca"
	}
	switch modo {
	case "off":
		return false, "repo_vivo.enabled = off"
	case "on":
		return true, "repo_vivo.enabled = on"
	default: // auto
		if !teamMode {
			return false, "auto: team_mode apagado"
		}
		if !remotoExiste {
			return false, "auto: el remoto declarado no existe en este checkout"
		}
		return true, "auto: team_mode activo y remoto empresarial presente"
	}
}
