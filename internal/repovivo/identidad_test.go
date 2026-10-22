package repovivo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

func TestSanitizarRefComponent(t *testing.T) {
	casos := []struct{ in, want string }{
		{"davantis-1", "davantis-1"},
		{"musubi-server.local", "musubi-server.local"},
		{"con espacios/y:raros", "con-espacios-y-raros"},
		{"--guiones--", "guiones"},
		{"...puntos...", "puntos"},
		{"rama..doble", "rama-doble"},
		{"algo.lock", "algo"},
		// git rechaza un componente que termine en ".lock". Recortarlo una sola vez no basta: estos
		// tres sobrevivían como "a.lock" con el saneo viejo (TrimSuffix antes de Trim), que es un
		// refname que git NO acepta. El saneo a punto fijo los lleva hasta "a".
		{"a.lock-", "a"},     // ".lock" tapado por basura de borde
		{"a.lock.", "a"},     // ídem con punto final
		{"a.lock.lock", "a"}, // ".lock" repetido: recortar uno destapa el otro
		{".lock", "x"},       // sólo el sufijo → vacío → "x"
		{"", "x"},
		{"@@@", "x"},
	}
	for _, c := range casos {
		if got := sanitizarRefComponent(c.in); got != c.want {
			t.Errorf("sanitizarRefComponent(%q) = %q, esperaba %q", c.in, got, c.want)
		}
	}
}

// TestNodoSiempreProduceRefnameValido es un test de PROPIEDAD que cierra el contrato del saneo contra
// el árbitro REAL: para cualquier texto hostil que pueda llegar a repo_vivo.nodo —o al basename de un
// checkout—, la ref derivada refs/vivo/<nodo>/<slug> tiene que ser un refname que git ACEPTE. Sin esto,
// un override como "a.lock-" sobrevivía como "a.lock", `git push` lo rechazaba (fatal: invalid refspec)
// y la zona en vivo fallaba muda en cada tick. No afirmamos la forma del componente: validamos la ref
// COMPLETA con `git check-ref-format`, que es quien decide de verdad en el push.
func TestNodoSiempreProduceRefnameValido(t *testing.T) {
	// Un slug real y válido: el riesgo vive en el <nodo>, pero validamos la ref entera igual.
	slug := slugDeCheckout("/home/davantis/musubi")
	hostiles := []string{
		"a.lock-", "a.lock.", "a.lock.lock", ".lock", "mi.lock-x", // el sufijo .lock en sus formas
		"../../x", "...", "..", "@@@", "con espacios", "rama..doble", // bordes, puntos y chars raros
		"davantis-1", "Davantis", "musubi-server.local", // los nombres reales del par davantis/davantis-1
	}
	for _, in := range hostiles {
		ref := refDeCheckout(nodo(in), slug)
		cmd := guiones.Herramienta(t, "git", "check-ref-format", ref)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("nodo(%q) → %q: git check-ref-format lo RECHAZA (%v)\n%s", in, ref, err, out)
		}
	}
}

func TestSlugEstableYUnico(t *testing.T) {
	a1 := slugDeCheckout("/home/davantis/musubi")
	a2 := slugDeCheckout("/home/davantis/musubi/") // mismo path, con barra: Clean lo iguala
	if a1 != a2 {
		t.Errorf("el slug no es estable: %q vs %q", a1, a2)
	}
	b := slugDeCheckout("/home/davantis/musubi-wt/otro")
	if a1 == b {
		t.Error("dos checkouts distintos dieron el mismo slug")
	}
	// mismo basename en carpetas distintas: deben diferir por el hash
	c1 := slugDeCheckout("/a/musubi")
	c2 := slugDeCheckout("/b/musubi")
	if c1 == c2 {
		t.Error("mismo basename en paths distintos no debería colisionar")
	}
	if !strings.HasPrefix(a1, "musubi-") {
		t.Errorf("el slug debería llevar el basename legible adelante: %q", a1)
	}
}

func TestRefDeCheckout(t *testing.T) {
	ref := refDeCheckout("davantis-1", "musubi-a1b2c3d4")
	if ref != "refs/vivo/davantis-1/musubi-a1b2c3d4" {
		t.Errorf("ref = %q", ref)
	}
	if !strings.HasPrefix(ref, prefijoRefVivo) {
		t.Errorf("la ref debe estar bajo %s", prefijoRefVivo)
	}
}

// TestNodo fija la identidad por máquina: un override de repo_vivo.nodo MANDA (sanitizado), y es lo
// que distingue dos máquinas con el mismo hostname (el caso davantis/davantis-1, ambas "Davantis").
// Sin override (vacío o sólo espacios) cae al hostname sanitizado.
func TestNodo(t *testing.T) {
	if got := nodo("davantis-1"); got != "davantis-1" {
		t.Errorf(`nodo("davantis-1") = %q, esperaba "davantis-1"`, got)
	}
	// el override se sanitiza igual que cualquier componente de refname
	if got := nodo("  mi nodo raro  "); got != "mi-nodo-raro" {
		t.Errorf(`nodo(" mi nodo raro ") = %q, esperaba "mi-nodo-raro"`, got)
	}
	// sin override: hostname sanitizado. Es el fallback, y debe coincidir byte a byte con lo que
	// derivaría Arrancar, o el poll miraría una ref que nunca se crea.
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		want := sanitizarRefComponent(h)
		if got := nodo(""); got != want {
			t.Errorf(`nodo("") = %q, esperaba el hostname sanitizado %q`, got, want)
		}
		if got := nodo("   "); got != want {
			t.Errorf("nodo(sólo espacios) = %q, esperaba caer al hostname %q", got, want)
		}
	}
}

// TestHostDeURL fija el parser LÉXICO del host, incluidas las cuatro formas que una guarda que
// sólo pelaba prefijos fijos y cortaba en el PRIMER '@' dejaba eludir: el userinfo con varios '@'
// (ssh://a@b@h → gana el último), el esquema compuesto (git+ssh://, ssh+git://), el puerto, y el
// FQDN absoluto con punto final (github.com.).
func TestHostDeURL(t *testing.T) {
	casos := []struct{ in, want string }{
		{"https://github.com/u/r.git", "github.com"},
		{"git@github.com:u/r.git", "github.com"},
		{"https://user@github.com/u/r", "github.com"},
		{"ssh://git@github.com/u/r", "github.com"},
		{"ssh://a@b@github.com/u/r", "github.com"},             // userinfo con 2 '@': gana el ÚLTIMO
		{"git+ssh://github.com/acme/altura.git", "github.com"}, // esquema compuesto
		{"ssh+git://github.com/x", "github.com"},
		{"github.com.", "github.com"},                                                           // FQDN absoluto (punto final)
		{"https://musubi-server.tail89e295.ts.net:3000/u/r", "musubi-server.tail89e295.ts.net"}, // puerto
		{"git@forgejo.local:u/r", "forgejo.local"},
		{"https://github.empresa.com/u/r", "github.empresa.com"},
		{"https://codeberg.org/u/r", "codeberg.org"},
		{"", ""},
	}
	for _, c := range casos {
		if got := hostDeURL(c.in); got != c.want {
			t.Errorf("hostDeURL(%q) = %q, esperaba %q", c.in, got, c.want)
		}
	}
}

// TestEsHostGitHub fija la comparación: github.com y sus subdominios cuentan; un dominio que apenas
// contiene "github" (github.empresa.com) o lo lleva pegado sin punto (notgithub.com, mygithub.com) NO.
func TestEsHostGitHub(t *testing.T) {
	for _, h := range []string{"github.com", "gist.github.com", "codeload.github.com"} {
		if !esHostGitHub(h) {
			t.Errorf("esHostGitHub(%q) = false, esperaba true", h)
		}
	}
	for _, h := range []string{"github.empresa.com", "notgithub.com", "mygithub.com", "forgejo.local", "", "codeberg.org"} {
		if esHostGitHub(h) {
			t.Errorf("esHostGitHub(%q) = true, esperaba false", h)
		}
	}
}

// TestDestinoEsGitHubResuelveInsteadOf prueba que la guarda RESUELVE el destino real antes de juzgar:
// un repo_vivo.remote escrito como alias insteadOf ("gh:acme/x" con url."git@github.com:".insteadOf
// "gh:") parece inofensivo por el string crudo, pero `git ls-remote --get-url` (sin red) lo expande a
// github.com. Es justo el camino que una guarda léxica no veía.
func TestDestinoEsGitHubResuelveInsteadOf(t *testing.T) {
	dir := repoEnVivo(t)
	g := nuevoGitRunner(dir)
	ctx := context.Background()
	runGit(t, dir, "config", "url.git@github.com:.insteadOf", "gh:")
	if !destinoEsGitHub(ctx, g, "gh:acme/altura.git") {
		t.Error("no resolvió el insteadOf a github.com (gh: → git@github.com:)")
	}
	// control: un destino que de verdad va a forgejo NO debe marcarse como github.
	if destinoEsGitHub(ctx, g, "https://forgejo.local/u/r") {
		t.Error("marcó forgejo como github")
	}
}

// TestHostnameDeSalidaSSH fija el parser de la salida de `ssh -G`, sin correr ssh: toma la clave
// `hostname`, la normaliza a minúsculas y cae al valor por defecto cuando no aparece.
func TestHostnameDeSalidaSSH(t *testing.T) {
	real := "user git\nhostname github.com\nport 22\nidentityfile ~/.ssh/id_ed25519\n"
	if got := hostnameDeSalidaSSH(real, "x"); got != "github.com" {
		t.Errorf("hostnameDeSalidaSSH(real) = %q, esperaba github.com", got)
	}
	if got := hostnameDeSalidaSSH("hostname GitHub.COM\n", "x"); got != "github.com" {
		t.Errorf("no normalizó a minúsculas: %q", got)
	}
	if got := hostnameDeSalidaSSH("user git\nport 22\n", "forgejo.local"); got != "forgejo.local" {
		t.Errorf("sin hostname debía caer al porDefecto, dio %q", got)
	}
}

// TestDestinoEsGitHubResuelveAliasSSH prueba el caso que mordió de verdad (incidente ef1caef6): un
// alias multi-cuenta de ~/.ssh/config ("git@gh-trabajo:...") cuyo HostName real es github.com. Sólo
// `ssh -G` (que lee la config sin conectar) lo destapa. OpenSSH ignora $HOME para hallar el config (lo
// resuelve por el passwd), así que se inyecta un config aislado con -F reemplazando salidaSSHMenosG;
// en producción corre `ssh -G` a secas.
func TestDestinoEsGitHubResuelveAliasSSH(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh no está en el PATH: no se puede resolver el alias con ssh -G")
	}
	dir := repoEnVivo(t)
	g := nuevoGitRunner(dir)
	ctx := context.Background()

	sshCfg := filepath.Join(t.TempDir(), "config")
	cfg := "Host gh-trabajo\n    HostName github.com\n    User git\nHost forge\n    HostName forgejo.local\n"
	if err := os.WriteFile(sshCfg, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	viejo := salidaSSHMenosG
	t.Cleanup(func() { salidaSSHMenosG = viejo })
	salidaSSHMenosG = func(ctx context.Context, host string) (string, error) {
		out, err := guiones.HerramientaCtx(ctx, t, "ssh", "-F", sshCfg, "-G", host).Output()
		return string(out), err
	}

	if !destinoEsGitHub(ctx, g, "git@gh-trabajo:acme/altura.git") {
		t.Error("no resolvió el alias ssh gh-trabajo → github.com (ssh -G)")
	}
	// control: un alias que resuelve a forgejo NO debe marcarse como github.
	if destinoEsGitHub(ctx, g, "git@forge:u/r") {
		t.Error("marcó un alias ssh que resuelve a forgejo como github")
	}
}

func TestDebeEncender(t *testing.T) {
	casos := []struct {
		nombre      string
		modo        string
		teamMode    bool
		apunta      bool // el remoto terminaría empujando a github (guarda dura)
		existe      bool
		wantEnc     bool
		wantRazonEn string
	}{
		{"off nunca", "off", true, false, true, false, "off"},
		{"on siempre", "on", false, false, true, true, "on"},
		{"on pero github: guarda dura", "on", false, true, true, false, "github.com"},
		{"auto con team y remoto", "auto", true, false, true, true, "team_mode activo"},
		{"auto sin team", "auto", false, false, true, false, "team_mode apagado"},
		{"auto sin remoto", "auto", true, false, false, false, "no existe"},
		{"auto con github: guarda dura", "auto", true, true, true, false, "github.com"},
	}
	for _, c := range casos {
		enc, razon := debeEncender(c.modo, c.teamMode, c.apunta, c.existe)
		if enc != c.wantEnc {
			t.Errorf("%s: debeEncender=%v, esperaba %v (razón %q)", c.nombre, enc, c.wantEnc, razon)
		}
		if !strings.Contains(razon, c.wantRazonEn) {
			t.Errorf("%s: razón %q no contiene %q", c.nombre, razon, c.wantRazonEn)
		}
	}
}

// TestRemotoApuntaAGitHub fija la guarda-github de M7 contra los TRES caminos por los que `git push`
// podía filtrar al espejo público y la versión vieja (que miraba sólo la 1ª URL de fetch) dejaba
// pasar: una 2ª URL en github, un pushurl a github, y una URL directa de github en repo_vivo.remote
// (que ni siquiera es un remoto declarado). Usa repos git reales, no stubs.
func TestRemotoApuntaAGitHub(t *testing.T) {
	dir := repoEnVivo(t)
	g := nuevoGitRunner(dir)
	ctx := context.Background()

	// (1) remoto declarado con una sola URL en forgejo: NO apunta a github; existe.
	runGit(t, dir, "remote", "add", "central", "https://forgejo.local/u/r")
	if apunta, existe := remotoApuntaAGitHub(ctx, g, "central"); apunta || !existe {
		t.Errorf("(1) forgejo: (apunta,existe)=(%v,%v), esperaba (false,true)", apunta, existe)
	}

	// (2) HOLE 2ª URL: una segunda URL (de fetch) en github → `git push central` empuja a AMBAS, así
	//     que DEBE apuntar. La guarda vieja, mirando sólo la 1ª URL, no lo veía.
	runGit(t, dir, "remote", "set-url", "--add", "central", "https://github.com/u/r.git")
	if apunta, _ := remotoApuntaAGitHub(ctx, g, "central"); !apunta {
		t.Error("(2) 2ª URL en github: apunta=false; la guarda miró sólo la 1ª URL del remoto")
	}

	// (3) HOLE pushurl: fetch en forgejo pero un pushurl a github → el push iría a github, no al
	//     fetch. Sólo `get-url --push` lo ve.
	runGit(t, dir, "remote", "add", "soloforge", "https://forgejo.local/a/b")
	runGit(t, dir, "remote", "set-url", "--push", "soloforge", "https://github.com/a/b.git")
	if apunta, _ := remotoApuntaAGitHub(ctx, g, "soloforge"); !apunta {
		t.Error("(3) pushurl github: apunta=false; la guarda miró la URL de fetch, no la de push")
	}

	// (4) HOLE URL directa: repo_vivo.remote es una URL de github, no un remoto declarado →
	//     `git push <url>` empujaría ahí directo → DEBE apuntar, con existe=false.
	if apunta, existe := remotoApuntaAGitHub(ctx, g, "https://github.com/u/r.git"); !apunta || existe {
		t.Errorf("(4) URL directa github: (apunta,existe)=(%v,%v), esperaba (true,false)", apunta, existe)
	}

	// (5) remoto inexistente y que no es URL de github: ni apunta ni existe (no arranca por 'auto',
	//     pero no por la guarda dura). Cubre también el valor que parece flag.
	if apunta, existe := remotoApuntaAGitHub(ctx, g, "noexiste"); apunta || existe {
		t.Errorf("(5) inexistente: (apunta,existe)=(%v,%v), esperaba (false,false)", apunta, existe)
	}
	if apunta, existe := remotoApuntaAGitHub(ctx, g, "--flag"); apunta || existe {
		t.Errorf("(5b) flag: (apunta,existe)=(%v,%v), esperaba (false,false)", apunta, existe)
	}
}
