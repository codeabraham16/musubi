package mcp

// despliegue_checksum_test.go — que nada que este repo BAJE de la red termine instalado sin que
// alguien haya comparado su sha256.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL CABO: EL BINARIO DEL CEREBRO SE INSTALABA SIN VERIFICAR, EN SILENCIO, SI EL .sha256 NO BAJABA
//
// `deploy/install-musubi-brain.sh` tenía, en el paso 1:
//
//	if curl -fsSL "$URL.sha256" -o "$tmpsha" && [ -s "$tmpsha" ]; then
//	  ... [ "$want" = "$got" ] || die "Checksum no coincide"
//	fi
//	install -m 0755 "$tmp" "$BIN"
//
// SIN `else`. Si el `.sha256` no bajaba —red, un 404 de un release que no lo publicó, un proxy que
// contesta 200 con HTML— el `if` no entraba, nadie comparaba nada, y el `install` se hacía IGUAL.
// Fail-OPEN sobre el archivo que después corre como servicio en el cerebro central.
//
// Y ERA EL HERMANO SIN LA GUARDA, el defecto dominante de este repo: en el MISMO archivo, los
// otros dos que baja —`musubi-backup.sh` y `redesplegar-cerebro.sh`— tienen pin duro y mueren si
// no coincide, y el comentario del segundo dice, sobre un guion MENOS privilegiado que el binario,
// que es «el peor archivo del despliegue para instalar sin verificar».
//
// POR QUÉ ESTAS PRUEBAS CORREN EL GUION Y NO LO GREPEAN. Preguntar `strings.Contains(guion, "die")`
// —o buscar un `else`— lo satisface un comentario, un mensaje de error o la línea vecina: es
// exactamente lo que pasó el 2026-09-05, siete guardas en verde sobre su propio sabotaje. Acá se
// EXTRAE el bloque que decide, se le da un `curl` de mentira, y se mira UNA sola cosa: si el
// archivo apareció en el destino. Esa es la consecuencia, no el texto que la acompaña.
//
// Y CADA PRUEBA TIENE SU LADO VERDE: con el checksum bueno el archivo TIENE que quedar instalado.
// Sin ese lado, un arnés roto (un `curl` de mentira que falla siempre, un bloque que no se extrae)
// daría verde sin haber instalado nunca nada, y «no pude medir» se leería como «medí y está bien».
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// leerGuionDeDespliegue devuelve el texto de un guion de deploy/ (ruta relativa a deploy/).
func leerGuionDeDespliegue(t *testing.T, rel string) string {
	t.Helper()
	p := filepath.Join("..", "..", "deploy", rel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("no se pudo leer deploy/%s: %v", rel, err)
	}
	return string(b)
}

// bloqueEntreMarcas recorta el trozo del guion que va de `desde` (incluido) hasta `hasta`
// (excluido). Si alguna marca no está, la prueba MUERE en vez de correr sobre un bloque vacío:
// un arnés que no encuentra qué ejecutar no puede dar verde.
func bloqueEntreMarcas(t *testing.T, guion, rel, desde, hasta string) string {
	t.Helper()
	i := strings.Index(guion, desde)
	if i < 0 {
		t.Fatalf("deploy/%s ya no tiene la marca %q: el guion cambió de forma y esta prueba dejó de "+
			"mirar donde se decide. Reapuntala antes de creerle a un verde", rel, desde)
	}
	resto := guion[i:]
	j := strings.Index(resto, hasta)
	if j < 0 {
		t.Fatalf("deploy/%s tiene %q pero no %q después: no puedo acotar el bloque que decide", rel, desde, hasta)
	}
	return resto[:j]
}

// funcionesDeSalida extrae las definiciones REALES de log/ok/die/aviso del guion. Se usan las del
// guion y no unas propias justamente porque lo que se está probando es que el camino termine en su
// `die` —el que hace `exit 1`—: si alguien lo convirtiera en un aviso que sigue de largo, el arnés
// tiene que notarlo.
func funcionesDeSalida(t *testing.T, guion, rel string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^(?:log|ok|die|aviso)\(\)\{.*$`)
	defs := re.FindAllString(guion, -1)
	if !strings.Contains(strings.Join(defs, "\n"), "die(){") {
		t.Fatalf("deploy/%s no define `die(){...}` con la forma de siempre: sin su salida real este "+
			"arnés no puede distinguir «frenó» de «siguió»", rel)
	}
	return strings.Join(defs, "\n")
}

// escribirStub deja un ejecutable de mentira en dir con ese nombre y ese cuerpo bash.
func escribirStub(t *testing.T, dir, nombre, cuerpo string) {
	t.Helper()
	p := filepath.Join(dir, nombre)
	if err := os.WriteFile(p, []byte("#!/usr/bin/env bash\n"+cuerpo), 0o755); err != nil {
		t.Fatalf("no se pudo escribir el stub %s: %v", nombre, err)
	}
}

// curlDeMentiraDelCerebro: sirve el "binario" desde $CARGA y decide qué contestar al `.sha256`
// según $MODO_SHA. Es la ÚNICA pieza simulada del paso 1 — el `sha256sum`, el `install`, el
// `mktemp` y el control de flujo de bash son los de verdad.
const curlDeMentiraDelCerebro = `
destino=""; url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) destino="$2"; shift 2 ;;
    -*) shift ;;
    *)  url="$1"; shift ;;
  esac
done
[[ -n "$destino" ]] || { echo "stub curl: me llamaron sin -o" >&2; exit 90; }
if [[ "$url" == *.sha256 ]]; then
  case "${MODO_SHA:-bueno}" in
    falla) exit 22 ;;
    vacio) : > "$destino" ;;
    html)  printf '<html><body>Portal cautivo: inicia sesion</body></html>\n' > "$destino" ;;
    otro)  printf '0000000000000000000000000000000000000000000000000000000000000000  musubi-linux-amd64\n' > "$destino" ;;
    bueno) printf '%s  musubi-linux-amd64\n' "$SHA_BUENO" > "$destino" ;;
    *) echo "stub curl: MODO_SHA desconocido" >&2; exit 91 ;;
  esac
  exit 0
fi
cp "$CARGA" "$destino"
`

// TestElBinarioDelCerebroNoSeInstalaSinVerificar — la guarda que cierra el cabo.
//
// Corre el paso 1 de verdad, con `curl` simulado, y mira si `$BIN` quedó escrito. Con el arreglo
// revertido (el `if` sin `else`), los cuatro casos que no pueden verificar instalan igual y esta
// prueba se pone roja en los cuatro.
func TestElBinarioDelCerebroNoSeInstalaSinVerificar(t *testing.T) {
	const rel = "install-musubi-brain.sh"
	guion := leerGuionDeDespliegue(t, rel)
	bloque := bloqueEntreMarcas(t, guion, rel, "# ── 1. Binario", "# ── 2. Workspace")
	salidas := funcionesDeSalida(t, guion, rel)

	// El "binario" descargado es un guion mínimo porque el paso 1 lo EJECUTA al final
	// (`ok "Binario instalado: $("$BIN" version)"`). Que corra es parte de que el camino verde
	// llegue hasta el final de verdad.
	const carga = "#!/bin/sh\necho \"musubi 0.0.0-de-prueba\"\n"
	shaCarga := fmt.Sprintf("%x", sha256.Sum256([]byte(carga)))
	const shaCeros = "0000000000000000000000000000000000000000000000000000000000000000"

	casos := []struct {
		nombre  string
		modoSha string // qué hace el curl del .sha256
		binSha  string // MUSUBI_BIN_SHA256
		instala bool
		porque  string
	}{
		{"el .sha256 no baja", "falla", "", false,
			"el .sha256 no bajó: nadie verificó nada y el binario del cerebro quedó instalado igual. Es EXACTAMENTE el cabo: un `if` sin `else` que deja pasar el `install`"},
		{"el .sha256 baja vacío", "vacio", "", false,
			"el .sha256 llegó vacío —un 200 sin cuerpo, un disco lleno— y se instaló sin comparar nada"},
		{"el .sha256 trae una página HTML", "html", "", false,
			"lo que llegó como .sha256 no era un sha256 (un portal cautivo contestando 200). Un `want` sin forma de sha256 significa «no pude medir», no «medí y está bien»"},
		{"el .sha256 no coincide", "otro", "", false,
			"el sha del binario NO coincidía con el publicado y se instaló igual"},
		{"el .sha256 coincide", "bueno", "", true,
			"el checksum coincidía y aun así el binario NO quedó instalado: el camino verde está roto, y con él el valor de los rojos de arriba"},
		{"el operador da el sha y coincide", "falla", shaCarga, true,
			"MUSUBI_BIN_SHA256 traía el sha correcto y aun así no se instaló: la única salida que le queda al operador cuando el .sha256 no está no funciona, y eso empuja a que alguien reabra el fail-open"},
		{"el operador da el sha en mayúsculas", "falla", strings.ToUpper(shaCarga), true,
			"MUSUBI_BIN_SHA256 con el sha correcto en mayúsculas se rechazó: es el mismo número, y un rechazo acá manda al operador a buscar un problema que no existe"},
		{"el operador da un sha equivocado", "falla", shaCeros, false,
			"MUSUBI_BIN_SHA256 no coincidía con el binario y se instaló igual"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			stubs := filepath.Join(dir, "stubs")
			if err := os.MkdirAll(stubs, 0o755); err != nil {
				t.Fatal(err)
			}
			escribirStub(t, stubs, "curl", curlDeMentiraDelCerebro)

			cargaP := filepath.Join(dir, "carga")
			if err := os.WriteFile(cargaP, []byte(carga), 0o644); err != nil {
				t.Fatal(err)
			}
			destino := filepath.Join(dir, "musubi")

			arnes := strings.Join([]string{
				"#!/usr/bin/env bash",
				"set -euo pipefail",
				"export PATH=" + shQuote(stubs) + `:"$PATH"`,
				salidas,
				"ARCH=amd64",
				"MUSUBI_REPO=prueba/musubi",
				"MUSUBI_VERSION=latest",
				"BIN=" + shQuote(destino),
				bloque,
			}, "\n")
			arnesP := filepath.Join(dir, "arnes.sh")
			if err := os.WriteFile(arnesP, []byte(arnes), 0o755); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("bash", arnesP)
			cmd.Env = append(os.Environ(),
				"MODO_SHA="+c.modoSha,
				"SHA_BUENO="+shaCarga,
				"CARGA="+cargaP,
			)
			if c.binSha != "" {
				cmd.Env = append(cmd.Env, "MUSUBI_BIN_SHA256="+c.binSha)
			}
			var salida bytes.Buffer
			cmd.Stdout = &salida
			cmd.Stderr = &salida
			err := cmd.Run()

			cuerpo, errLeer := os.ReadFile(destino)
			instalado := errLeer == nil

			if instalado != c.instala {
				t.Errorf("%s\n  se instaló el binario: %v (se esperaba %v)\n  el guion salió con: %v\n  salida:\n%s",
					c.porque, instalado, c.instala, err, salida.String())
			}
			if c.instala {
				if err != nil {
					t.Errorf("el camino verificado tiene que terminar bien y salió con %v\n%s", err, salida.String())
				}
				if instalado && string(cuerpo) != carga {
					t.Errorf("se instaló algo distinto de lo que se descargó")
				}
			} else if err == nil {
				t.Errorf("%s\n  el guion salió con código 0: no frenó\n  salida:\n%s", c.porque, salida.String())
			}
		})
	}
}

// TestElRelayDeRustdeskNoSeInstalaSinVerificar — el hermano que la barrida encontró.
//
// `deploy/rustdesk/install-rustdesk-relay.sh` bajaba el zip oficial de rustdesk-server y instalaba
// `hbbs`/`hbbr` —que quedan como unidades systemd de este servidor— SIN comparar nada: ni pin, ni
// checksum publicado, ni nada. Es el mismo fail-open que el del cerebro, sobre un origen que
// además no es nuestro. Rustdesk no publica checksums en sus releases (medido el 2026-09-10: los
// 18 assets del tag 1.1.14 son .deb y .zip, ninguno .sha256), así que el pin en el guion es la
// única opción — el mismo criterio que BACKUP_SHA256 en el instalador del cerebro.
//
// La prueba corre el bloque de binarios con `curl`/`useradd`/`chown` simulados y `unzip`,
// `sha256sum`, `find` e `install` de verdad, y mira si `hbbs` apareció en el destino.
func TestElRelayDeRustdeskNoSeInstalaSinVerificar(t *testing.T) {
	const rel = "rustdesk/install-rustdesk-relay.sh"
	guion := leerGuionDeDespliegue(t, rel)
	bloque := bloqueEntreMarcas(t, guion, rel, "# ── Binarios", "# ── systemd")
	salidas := funcionesDeSalida(t, guion, rel)

	zipFalso := zipConRelay(t)
	shaZip := fmt.Sprintf("%x", sha256.Sum256(zipFalso))

	// El curl de mentira anota que lo llamaron: así se puede afirmar que el caso «no hay pin»
	// frena ANTES de bajar seis megas, y no después.
	const curlDeMentiraDelRelay = `
destino=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) destino="$2"; shift 2 ;;
    -*) shift ;;
    *)  shift ;;
  esac
done
: > "$MARCA_CURL"
[[ -n "$destino" ]] || exit 90
cp "$CARGA" "$destino"
`

	casos := []struct {
		nombre  string
		version string // qué versión pide el guion
		sha     string // RUSTDESK_SHA256
		instala bool
		bajo    bool // ¿tenía que llegar a descargar?
		porque  string
	}{
		{"el zip no coincide con el pin de la versión", "1.1.14", "", false, true,
			"el zip bajado NO coincide con el pin de PINES_RUSTDESK y hbbs/hbbr quedaron instalados igual como servicios de este servidor"},
		{"una versión sin pin", "9.9.9-inexistente", "", false, false,
			"se pidió una versión que no tiene pin y el guion siguió: instalar sin ningún número contra el que comparar es el fail-open original"},
		{"el operador da el sha y coincide", "9.9.9-inexistente", shaZip, true, true,
			"RUSTDESK_SHA256 traía el sha correcto y aun así no se instaló: la salida del operador para una versión sin fila no funciona"},
		{"el operador da un sha equivocado", "9.9.9-inexistente", "0000000000000000000000000000000000000000000000000000000000000000", false, true,
			"RUSTDESK_SHA256 no coincidía con el zip y se instaló igual"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			stubs := filepath.Join(dir, "stubs")
			if err := os.MkdirAll(stubs, 0o755); err != nil {
				t.Fatal(err)
			}
			escribirStub(t, stubs, "curl", curlDeMentiraDelRelay)
			escribirStub(t, stubs, "useradd", "exit 0\n")
			escribirStub(t, stubs, "chown", "exit 0\n")

			cargaP := filepath.Join(dir, "relay.zip")
			if err := os.WriteFile(cargaP, zipFalso, 0o644); err != nil {
				t.Fatal(err)
			}
			marca := filepath.Join(dir, "curl-llamado")
			destino := filepath.Join(dir, "opt-rustdesk")

			arnes := strings.Join([]string{
				"#!/usr/bin/env bash",
				"set -euo pipefail",
				"export PATH=" + shQuote(stubs) + `:"$PATH"`,
				salidas,
				"VERSION=" + shQuote(c.version),
				"DESTINO=" + shQuote(destino),
				"USUARIO=rustdesk-de-prueba",
				bloque,
			}, "\n")
			arnesP := filepath.Join(dir, "arnes.sh")
			if err := os.WriteFile(arnesP, []byte(arnes), 0o755); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("bash", arnesP)
			cmd.Env = append(os.Environ(), "CARGA="+cargaP, "MARCA_CURL="+marca)
			if c.sha != "" {
				cmd.Env = append(cmd.Env, "RUSTDESK_SHA256="+c.sha)
			}
			var salida bytes.Buffer
			cmd.Stdout = &salida
			cmd.Stderr = &salida
			err := cmd.Run()

			_, errHbbs := os.Stat(filepath.Join(destino, "hbbs"))
			instalado := errHbbs == nil
			_, errMarca := os.Stat(marca)
			bajo := errMarca == nil

			if instalado != c.instala {
				t.Errorf("%s\n  se instaló hbbs: %v (se esperaba %v)\n  el guion salió con: %v\n  salida:\n%s",
					c.porque, instalado, c.instala, err, salida.String())
			}
			if bajo != c.bajo {
				t.Errorf("descargó el paquete: %v (se esperaba %v). Sin pin y sin RUSTDESK_SHA256 el guion "+
					"tiene que frenar ANTES de bajar el zip: el operador se entera al principio y no al final.\n%s",
					bajo, c.bajo, salida.String())
			}
			if c.instala && err != nil {
				t.Errorf("el camino verificado tiene que terminar bien y salió con %v\n%s", err, salida.String())
			}
			if !c.instala && err == nil {
				t.Errorf("%s\n  el guion salió con código 0: no frenó\n  salida:\n%s", c.porque, salida.String())
			}
		})
	}
}

// TestElPinDelRelayCubreTodasLasArquitecturasQueElGuionOfrece — que la tabla no se quede corta.
//
// El guion elige el paquete con un `case` sobre `uname -m`. Si alguien agrega una arquitectura, o
// sube la versión por default, y no toca PINES_RUSTDESK, el guion muere en la máquina de quien
// esté instalando —fail-closed, que es lo correcto, pero en el peor momento—. La lista de paquetes
// NO se escribe acá a mano: se DERIVA del `case` del propio guion, que es lo que decide.
func TestElPinDelRelayCubreTodasLasArquitecturasQueElGuionOfrece(t *testing.T) {
	const rel = "rustdesk/install-rustdesk-relay.sh"
	guion := leerGuionDeDespliegue(t, rel)

	mv := regexp.MustCompile(`(?m)^VERSION="\$\{RUSTDESK_SERVER_VERSION:-([^}]+)\}"`).FindStringSubmatch(guion)
	if mv == nil {
		t.Fatal("el guion ya no declara VERSION=\"${RUSTDESK_SERVER_VERSION:-<v>}\": esta guarda no sabe " +
			"qué versión es la de por default, así que no puede comprobar que tenga pin")
	}
	version := mv[1]

	paquetes := regexp.MustCompile(`PAQUETE="([^"]+\.zip)"`).FindAllStringSubmatch(guion, -1)
	if len(paquetes) == 0 {
		t.Fatal("no encontré ninguna asignación de PAQUETE=\"...zip\" en el guion: cambió de forma y esta " +
			"guarda quedó mirando donde ya no se elige el paquete")
	}

	tabla := regexp.MustCompile(`(?s)PINES_RUSTDESK="\n(.*?)\n"`).FindStringSubmatch(guion)
	if tabla == nil {
		t.Fatal("el guion no declara PINES_RUSTDESK=\"...\": o se quitó la verificación del paquete del " +
			"relay —y entonces se instalan como servicio systemd los binarios que vengan— o cambió de forma")
	}
	pin := map[string]string{}
	for _, linea := range strings.Split(tabla[1], "\n") {
		campos := strings.Fields(linea)
		if len(campos) == 0 {
			continue
		}
		if len(campos) != 3 {
			t.Errorf("fila de PINES_RUSTDESK con %d campos y se esperan 3 (version paquete sha256): %q", len(campos), linea)
			continue
		}
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(campos[2]) {
			t.Errorf("la fila %q no trae un sha256: un pin que no tiene forma de sha256 hace morir al "+
				"guion sin decir por qué", linea)
			continue
		}
		pin[campos[0]+" "+campos[1]] = campos[2]
	}

	comprobados := 0
	for _, p := range paquetes {
		clave := version + " " + p[1]
		if _, ok := pin[clave]; !ok {
			t.Errorf("el guion puede elegir %q para la versión por default %s y PINES_RUSTDESK no tiene "+
				"esa fila: en esa arquitectura el instalador muere sin instalar nada.\n"+
				"Arreglo: bajá el zip oficial, sacale el sha256 y agregá la fila `%s %s <sha256>`",
				p[1], version, version, p[1])
			continue
		}
		comprobados++
	}
	if comprobados == 0 {
		t.Fatal("no quedó ningún paquete comprobado: esta prueba estaría en verde sin haber mirado nada")
	}
	t.Logf("%d paquete(s) del relay con pin para la versión %s", comprobados, version)
}

// zipConRelay arma un zip con la misma forma que el oficial (arch/hbbs, arch/hbbr), para que el
// `unzip` y el `find ... -name hbbs` del guion sean los de verdad.
func zipConRelay(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, n := range []string{"amd64/hbbs", "amd64/hbbr"} {
		w, err := z.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("#!/bin/sh\necho " + filepath.Base(n) + " de prueba\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// shQuote entrecomilla para bash con comillas simples.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
