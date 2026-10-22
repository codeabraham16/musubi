// Package repovivo es la ZONA EN VIVO: sube fotos del working tree de un checkout al remoto
// empresarial (Forgejo), solas, cada pocos segundos, SIN tocar el índice, el HEAD ni las ramas
// del usuario. Tolera código roto —fotografía lo que haya— y es best-effort: si el central no
// está, el tick se descarta y la sesión ni se entera.
//
// Las piezas: fotografo (foto.go) captura el working tree por plumbing en un commit huérfano;
// transporte (transporte.go) lo publica bajo refs/vivo/<nodo>/<slug>, FUERA de refs/heads/ para
// que el espejo público de Altura nunca lo replique; identidad (identidad.go) deriva esa ref y
// decide el encendido, con una guarda DURA que impide arrancar si el remoto es github.com; y el
// observador (observador.go) es el bucle que ata todo con dedupe por árbol.
//
// Arrancar es la única entrada: internal/mcp la llama desde el daemon con la config del proyecto.
package repovivo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"musubi/internal/logx"
)

// Opciones es lo que el daemon pasa para la zona en vivo de un checkout. Dir es cualquier ruta
// dentro del checkout (el daemon pasa su projectPath); Remoto es el nombre del remoto git (no la
// URL); Modo es auto|on|off; TeamMode viene de la config de memoria del proyecto.
type Opciones struct {
	Dir           string
	Remoto        string
	Modo          string
	TeamMode      bool
	Intervalo     time.Duration
	SecretosExtra []string
	// Nodo es el nombre de ESTA máquina en la ref (refs/vivo/<nodo>/…), de repo_vivo.nodo. Vacío usa
	// el hostname; se declara para distinguir dos máquinas que comparten hostname (ver nodo()).
	Nodo string
}

// Arrancar decide si la zona en vivo debe correr para este checkout y, si corresponde, corre el
// bucle hasta que ctx muera (bloquea: el daemon la invoca en su propia goroutine). Devuelve
// (encendió, razón, error): la razón es para el log de arranque en las dos ramas (encendió o no).
// Un error sólo aparece cuando algo que debería andar falló al construirse (p. ej. Dir no es un
// repo, o la ref derivada es inválida); "no debe encender" NO es error.
func Arrancar(ctx context.Context, o Opciones) (bool, string, error) {
	git := nuevoGitRunner(o.Dir)

	// toplevel real del working tree: es la identidad por checkout (dos worktrees del mismo repo
	// tienen toplevels distintos → refs distintas). Si falla, Dir no es un repo git: no arranca.
	salida, err := git.run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return false, "no es un repo git", fmt.Errorf("rev-parse --show-toplevel en %s: %w", o.Dir, err)
	}
	toplevel := strings.TrimSpace(salida)

	// La guarda github se evalúa acá, al ARRANCAR, UNA vez. Una deriva en caliente —un `git remote
	// set-url` que repunte el remoto a github mientras el bucle corre— NO se re-chequea, y es a
	// propósito: la defensa contra que una foto llegue a producción es ESTRUCTURAL y no deriva —la ref
	// vive bajo refs/vivo/ (nuevoTransporte rechaza cualquier otra), fuera de refs/heads/+refs/tags/,
	// que es lo único que el espejo de Altura replica a github—. Esta guarda es defensa en profundidad
	// del arranque, no una invariante de runtime: re-resolver el destino (ls-remote, ssh -G) en cada
	// tick contradiría el diseño liviano y best-effort del bucle.
	apunta, existe := remotoApuntaAGitHub(ctx, git, o.Remoto)
	enc, razon := debeEncender(o.Modo, o.TeamMode, apunta, existe)
	if !enc {
		return false, razon, nil
	}

	foto, err := nuevoFotografo(o.Dir, o.SecretosExtra)
	if err != nil {
		return false, "no se pudo preparar el fotógrafo", err
	}
	ref := refDeCheckout(nodo(o.Nodo), slugDeCheckout(toplevel))
	trans, err := nuevoTransporte(foto.git, o.Remoto, ref)
	if err != nil {
		return false, "ref o remoto inválidos", err
	}

	intervalo := o.Intervalo
	if intervalo <= 0 {
		intervalo = 5 * time.Second
	}
	logx.Info("repo-vivo: encendido", "ref", ref, "remoto", o.Remoto, "razon", razon, "intervalo", intervalo.String())
	nuevoObservador(foto, trans, intervalo).correr(ctx)
	return true, razon, nil
}
