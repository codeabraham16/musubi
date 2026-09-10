package mcp

// despliegue_verificacion_corrida_test.go — RONDA 2: los bloques que instalan, CORRIDOS.
//
// El hermano de `despliegue_verificacion_forma_test.go`. Aquél mira la FORMA (que ningún `install`
// sea alcanzable sin haber comparado); éste EJECUTA el bloque de verdad y mira la CONSECUENCIA:
// si el archivo apareció en el destino. Los dos hacen falta y ninguno reemplaza al otro:
//
//   - la forma ve una vía de escape que existe pero está apagada —el `if` con `else` que no se
//     activa en la corrida de la prueba—;
//   - la corrida ve un fail-open que la forma no puede distinguir de un guion correcto, como un
//     `die` que en realidad no sale, o un pin vacío.
//
// LO QUE ESTE ARCHIVO AGREGA A LA RONDA 1:
//
//  1. LA BARRIDA DE ENTORNO. Se derivan del propio bloque TODAS las variables de entorno que lee y
//     que nadie le asigna —eso, por definición, es una entrada de afuera— y se corre el caso que
//     TIENE que frenar una vez por cada variable y por cada valor típico de encendido. Ninguna
//     puede convertir un camino que frena en uno que instala. La lista sale del guion: no hay un
//     `MUSUBI_SIN_VERIFICAR` escrito acá, porque el próximo se va a llamar distinto.
//  2. LOS PASOS 5b y 5c, que la ronda 1 declaró cubiertos y NO lo estaban. Sus únicas guardas
//     comparaban el pin contra el archivo del repo: ven que cambien los BYTES del guion, no que
//     desaparezca la comparación del instalador. Se les borró el `if` y el `die` enteros y el
//     paquete quedó verde.
//  3. EL INSTALADOR DE PROMETHEUS, al que se le borraron los dos `die` dejando el `curl`, el `awk`
//     y el `ok "Checksum verificado"`: imprimía que verificó sin verificar nada.
//  4. EL REDESPLIEGUE DEL CEREBRO con el sha esperado ADOPTANDO el del binario que le pasan.
//  5. QUE «NO PUDE MEDIR» FRENE ANTES DE MIRAR EL BINARIO, y no se disfrace de «no coincide».
//
// RONDA 3 — LO QUE LA BARRIDA NO MIRABA. Corría SIEMPRE con MODO_SHA=falla: ejercitaba «no pude
// medir» y nunca «medí y NO coincide». Son dos ramas distintas del guion, y una vía de escape
// puesta en la segunda le era invisible POR CONSTRUCCIÓN — medido: `&& [ -z
// "${MUSUBI_IGUAL_INSTALA:-}" ]` pegado a la comparación dejaba la barrida entera en verde. Ahora
// las dos barridas —la del paso 1 y la del relay— corren los DOS caminos que tienen que frenar.
// Y la guarda de forma del sha, que sólo miraba que el guion no nombrara el sha del binario, no
// veía que se DEGRADARA el regex (`{7,}` por `{64}`, o `.` por `[0-9a-f]`): se le agregaron las
// tres entradas que sólo pasan si el guion sigue exigiendo 64 hexadecimales exactos.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// Arnés común
// ─────────────────────────────────────────────────────────────────────────────────────────────

// stubInstall reemplaza a `install` en el PATH del arnés. Anota el destino y deja una copia en
// $CAJA, sin tocar la ruta real: la prueba no corre como root y los destinos de verdad son
// /usr/local/bin y /etc. Descarta SÓLO `-m`, `-o` y `-g` —los permisos y el dueño, que no deciden
// nada sobre la verificación— y conserva la única pregunta que importa: ¿se llamó, y con qué
// archivo?
const stubInstall = `
args=()
while (($#)); do
  case "$1" in
    -m|-o|-g) shift 2 ;;
    -d) shift ;;
    -*) shift ;;
    *) args+=("$1"); shift ;;
  esac
done
n=${#args[@]}
[[ $n -ge 2 ]] || { echo "stub install: me llamaron con $n rutas" >&2; exit 90; }
destino="${args[n-1]}"
origen="${args[0]}"
printf '%s\n' "$destino" >> "$CAJA/instalados"
cp "$origen" "$CAJA/$(basename "${destino%/}")"
`

// corridaDeBloque es una ejecución del arnés: el prólogo, el bloque real del guion y lo que se
// midió después.
type corridaDeBloque struct {
	instalados []string // destinos que recibieron un `install`
	salida     string
	err        error
	caja       string
}

func (c corridaDeBloque) instaloEn(destino string) bool {
	for _, d := range c.instalados {
		if strings.TrimSuffix(d, "/") == strings.TrimSuffix(destino, "/") {
			return true
		}
	}
	return false
}

func (c corridaDeBloque) instaloAlgo() bool { return len(c.instalados) > 0 }

// correrArnes escribe y corre un guion de arnés en un directorio propio.
func correrArnes(t *testing.T, dir string, lineas []string, entorno []string) corridaDeBloque {
	t.Helper()
	caja := filepath.Join(dir, "caja")
	if err := os.MkdirAll(caja, 0o755); err != nil {
		t.Fatal(err)
	}
	arnesP := filepath.Join(dir, "arnes.sh")
	if err := os.WriteFile(arnesP, []byte(strings.Join(lineas, "\n")+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", arnesP)
	cmd.Env = append(os.Environ(), append([]string{"CAJA=" + caja}, entorno...)...)
	var salida bytes.Buffer
	cmd.Stdout = &salida
	cmd.Stderr = &salida
	err := cmd.Run()

	c := corridaDeBloque{salida: salida.String(), err: err, caja: caja}
	if b, e := os.ReadFile(filepath.Join(caja, "instalados")); e == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if l != "" {
				c.instalados = append(c.instalados, l)
			}
		}
	}
	return c
}

// declaracionDelGuion extrae, TAL CUAL, la línea donde el guion declara una variable de primer
// nivel. El prólogo del arnés no reescribe los pines: los toma del archivo, así que si alguien
// vacía un pin la prueba lo corre con el pin vacío y se entera.
func declaracionDelGuion(t *testing.T, guion, rel, nombre string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(nombre) + `="[^"]*"$`)
	m := re.FindString(guion)
	if m == "" {
		t.Fatalf("deploy/%s ya no declara %s=\"...\" en una línea propia: o se quitó —y entonces no "+
			"hay contra qué comparar el guion que se instala— o cambió de forma y este arnés dejó de "+
			"correr lo que decide", rel, nombre)
	}
	return m
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// 1. LA BARRIDA DE ENTORNO
// ─────────────────────────────────────────────────────────────────────────────────────────────

// entradasDeAfuera devuelve los nombres de variable que el bloque LEE y que nadie le asigna, ni
// en el bloque ni en el prólogo del arnés. Ésas son, por definición, las entradas que vienen del
// entorno — y una vía de escape agregada a mano es siempre una de ellas.
//
// SE DERIVA DEL GUION. No hay una lista de nombres prohibidos acá: una lista de formas malas
// siempre le falta la próxima, que es la lección que este repo ya pagó.
func entradasDeAfuera(t *testing.T, bloque, prologo string) []string {
	t.Helper()
	asignadas := map[string]bool{}
	leidas := map[string]bool{}
	for _, fuente := range []struct {
		texto string
		leer  bool
	}{{prologo, false}, {bloque, true}} {
		toks, err := tokenizarBloque(fuente.texto)
		if err != nil {
			t.Fatalf("no pude tokenizar para derivar las entradas de entorno: %v", err)
		}
		for _, tk := range toks {
			if m := reAsignacionVar.FindStringSubmatch(tk.tok); m != nil {
				asignadas[m[1]] = true
			}
			if fuente.leer {
				for _, m := range reReferenciaVar.FindAllStringSubmatch(tk.tok, -1) {
					leidas[m[1]] = true
				}
			}
		}
	}
	var fuera []string
	for v := range leidas {
		if !asignadas[v] {
			fuera = append(fuera, v)
		}
	}
	sort.Strings(fuera)
	return fuera
}

// valoresDeEncendido — las formas en que alguien prende una bandera en un runbook.
var valoresDeEncendido = []string{"1", "0", "true", "yes", "si", "on"}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL PASO 1 DEL CEREBRO: la barrida, y «no pude medir» que frena antes de mirar el binario
// ─────────────────────────────────────────────────────────────────────────────────────────────

// arnesDelPaso1 arma el arnés del paso 1 de install-musubi-brain.sh y devuelve (prólogo, bloque).
func arnesDelPaso1(t *testing.T, dir, destino, stubs string) (string, string) {
	t.Helper()
	const rel = "install-musubi-brain.sh"
	guion := leerGuionDeDespliegue(t, rel)
	bloque := bloqueEntreMarcas(t, guion, rel, "# ── 1. Binario", "# ── 2. Workspace")
	prologo := strings.Join([]string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		"export PATH=" + shQuote(stubs) + `:"$PATH"`,
		funcionesDeSalida(t, guion, rel),
		"ARCH=amd64",
		"MUSUBI_REPO=prueba/musubi",
		"MUSUBI_VERSION=latest",
		"BIN=" + shQuote(destino),
	}, "\n")
	return prologo, bloque
}

// TestNingunaVariableDeEntornoSalteaLaVerificacionDelBinario — la barrida del paso 1.
//
// EL AGUJERO QUE CIERRA, MEDIDO: se envolvió toda la verificación en
// `if [ -z "${MUSUBI_SIN_VERIFICAR:-}" ]` con un `else` que sólo loguea «instalando sin
// verificar», y las ocho subpruebas de la ronda 1 siguieron en verde — porque ninguna exportaba
// esa variable. Y los tres lugares donde el guion afirma por escrito que no existe una variable
// para saltear la verificación seguían afirmándolo sin nada que lo sostuviera.
//
// Acá el caso que TIENE que frenar se corre una vez por cada variable de entorno que el bloque lee
// y nadie le asigna, y por cada valor con el que alguien prende una bandera. Si alguna instala, hay
// una vía de escape.
//
// Y SE CORRE POR LOS DOS CAMINOS QUE TIENEN QUE FRENAR, que es la fuga que esta ronda cierra. La
// barrida iba SIEMPRE con MODO_SHA=falla: ejercitaba «no pude medir» y NUNCA «medí y NO coincide».
// Los dos terminan en un `die`, pero son ramas distintas del guion, y cualquier vía de escape que
// viva en la segunda le era invisible POR CONSTRUCCIÓN. Medido: agregarle a la comparación un
// `&& [ -z "${MUSUBI_IGUAL_INSTALA:-}" ]` dejaba la barrida entera en verde, porque con el .sha256
// caído el guion muere antes de llegar a esa línea.
//
// El hermano es MODO_SHA=otro: el .sha256 baja, tiene forma de sha256 y NO es el del binario.
func TestNingunaVariableDeEntornoSalteaLaVerificacionDelBinario(t *testing.T) {
	const carga = "#!/bin/sh\necho \"musubi 0.0.0-de-prueba\"\n"
	shaCarga := fmt.Sprintf("%x", sha256.Sum256([]byte(carga)))

	preparar := func(t *testing.T) (dir, destino string, prologo, bloque string) {
		dir = t.TempDir()
		stubs := filepath.Join(dir, "stubs")
		if err := os.MkdirAll(stubs, 0o755); err != nil {
			t.Fatal(err)
		}
		escribirStub(t, stubs, "curl", curlDeMentiraDelCerebro)
		cargaP := filepath.Join(dir, "carga")
		if err := os.WriteFile(cargaP, []byte(carga), 0o644); err != nil {
			t.Fatal(err)
		}
		destino = filepath.Join(dir, "musubi")
		prologo, bloque = arnesDelPaso1(t, dir, destino, stubs)
		return dir, destino, prologo, bloque
	}

	// CONTROL VERDE: con el sha bueno el binario TIENE que quedar instalado. Sin este lado, un
	// arnés roto daría verde en toda la barrida sin haber instalado nunca nada.
	t.Run("control: con el checksum bueno instala", func(t *testing.T) {
		dir, destino, prologo, bloque := preparar(t)
		c := correrArnes(t, dir, []string{prologo, bloque},
			[]string{"MODO_SHA=bueno", "SHA_BUENO=" + shaCarga, "CARGA=" + filepath.Join(dir, "carga")})
		if _, err := os.Stat(destino); err != nil {
			t.Fatalf("el camino verificado no instaló nada: el arnés está roto y los rojos de abajo no "+
				"valdrían nada.\n%v\n%s", c.err, c.salida)
		}
	})

	_, _, prologo, bloque := preparar(t)
	candidatas := entradasDeAfuera(t, bloque, prologo)
	if len(candidatas) == 0 {
		t.Fatal("el bloque del paso 1 no lee NINGUNA variable de entorno según el análisis: eso es " +
			"imposible (lee MUSUBI_BIN_SHA256), así que la derivación se rompió y esta barrida " +
			"estaría en verde sin haber probado nada")
	}
	t.Logf("entradas de entorno derivadas del bloque: %v", candidatas)

	for _, m := range losDosCaminosQueFrenan {
		for _, v := range candidatas {
			for _, valor := range valoresDeEncendido {
				t.Run(m.modo+"/"+v+"="+valor, func(t *testing.T) {
					dir, destino, prologo, bloque := preparar(t)
					c := correrArnes(t, dir, []string{prologo, bloque}, []string{
						"MODO_SHA=" + m.modo,
						"SHA_BUENO=" + shaCarga,
						"CARGA=" + filepath.Join(dir, "carga"),
						v + "=" + valor,
					})
					if _, err := os.Stat(destino); err == nil {
						t.Errorf("con %s=%s y %s, el binario del cerebro QUEDÓ INSTALADO sin que nadie "+
							"comparara nada.\nEso es una vía de escape fail-open: una variable de entorno "+
							"que apaga la verificación. Termina copiada en un runbook y de ahí en todas "+
							"las máquinas, mientras el guion sigue diciendo que no existe.\n"+
							"La salida para el operador es DAR el sha (MUSUBI_BIN_SHA256), nunca saltear "+
							"la comprobación.\nsalida del guion:\n%s", v, valor, m.porque, c.salida)
					}
					if c.err == nil {
						t.Errorf("con %s=%s y %s el guion salió con código 0: no frenó.\n%s",
							v, valor, m.porque, c.salida)
					}
				})
			}
		}
	}
}

// losDosCaminosQueFrenan — las DOS respuestas distintas que el guion tiene que dar, y que hay que
// barrer por separado.
//
// «No pude medir» y «medí y no coincide» viven en ramas DISTINTAS del bloque. Barrer sólo la
// primera deja la segunda sin nadie mirando: una vía de escape puesta ahí no la ve ninguna corrida
// de esta prueba, y la de FORMA tampoco si la comparación sigue existiendo y dominando.
var losDosCaminosQueFrenan = []struct{ modo, porque string }{
	{"falla", "el .sha256 no bajó y el operador no dio el sha (NADIE puede verificar)"},
	{"otro", "el .sha256 bajó, tiene forma de sha256 y NO es el del binario (se midió y difiere)"},
}

// TestNingunaVariableDeEntornoSalteaLaVerificacionDelRelay — EL HERMANO de la barrida de arriba.
//
// El mismo sabotaje se midió en `deploy/rustdesk/install-rustdesk-relay.sh`, y ahí lo que se
// instala sin verificar son `hbbs`/`hbbr`, que quedan como unidades systemd de este servidor con
// lo que venga de un release ajeno. Cerrar el paso 1 y dejar éste abierto es exactamente la forma
// en que este repo pierde: la lección aprendida de un lado y no del hermano.
//
// Y ACÁ TAMBIÉN SE BARREN LOS DOS CAMINOS QUE FRENAN, por la misma razón que en el paso 1: una
// versión SIN fila en PINES_RUSTDESK y sin RUSTDESK_SHA256 es «no pude medir» —no hay ningún
// número contra el cual comparar, y se frena ANTES de bajar seis megas—; un RUSTDESK_SHA256 con
// forma de sha256 que no es el del zip es «medí y NO coincide», que es otra rama del guion. Barrer
// una sola deja la otra sin nadie mirando.
func TestNingunaVariableDeEntornoSalteaLaVerificacionDelRelay(t *testing.T) {
	const rel = "rustdesk/install-rustdesk-relay.sh"
	guion := leerGuionDeDespliegue(t, rel)
	bloque := bloqueEntreMarcas(t, guion, rel, "# ── Binarios", "# ── systemd")
	zipFalso := zipConRelay(t)

	preparar := func(t *testing.T, version string) (dir, destino, prologo string) {
		dir = t.TempDir()
		stubs := filepath.Join(dir, "stubs")
		if err := os.MkdirAll(stubs, 0o755); err != nil {
			t.Fatal(err)
		}
		cargaP := filepath.Join(dir, "relay.zip")
		if err := os.WriteFile(cargaP, zipFalso, 0o644); err != nil {
			t.Fatal(err)
		}
		escribirStub(t, stubs, "curl", `
destino=""
while (($#)); do case "$1" in -o) destino="$2"; shift 2 ;; -*) shift ;; *) shift ;; esac; done
[[ -n "$destino" ]] || exit 90
cp `+shQuote(cargaP)+` "$destino"
`)
		escribirStub(t, stubs, "useradd", "exit 0\n")
		escribirStub(t, stubs, "chown", "exit 0\n")
		destino = filepath.Join(dir, "opt-rustdesk")
		prologo = strings.Join([]string{
			"#!/usr/bin/env bash",
			"set -euo pipefail",
			"export PATH=" + shQuote(stubs) + `:"$PATH"`,
			funcionesDeSalida(t, guion, rel),
			"VERSION=" + shQuote(version),
			"DESTINO=" + shQuote(destino),
			"USUARIO=rustdesk-de-prueba",
		}, "\n")
		return dir, destino, prologo
	}

	// CONTROL VERDE: con el sha correcto dado por el operador, hbbs TIENE que quedar instalado.
	t.Run("control: con el sha del operador instala", func(t *testing.T) {
		dir, destino, prologo := preparar(t, "9.9.9-sin-fila")
		sha := fmt.Sprintf("%x", sha256.Sum256(zipFalso))
		correrArnes(t, dir, []string{prologo, bloque}, []string{"RUSTDESK_SHA256=" + sha})
		if _, err := os.Stat(filepath.Join(destino, "hbbs")); err != nil {
			t.Fatalf("el camino verificado no instaló hbbs: el arnés está roto y los rojos de abajo no "+
				"valdrían nada: %v", err)
		}
	})

	_, _, prologo := preparar(t, "9.9.9-sin-fila")
	candidatas := entradasDeAfuera(t, bloque, prologo)
	if len(candidatas) == 0 {
		t.Fatal("el bloque del relay no lee NINGUNA variable de entorno según el análisis: eso es " +
			"imposible (lee RUSTDESK_SHA256), así que la derivación se rompió")
	}
	t.Logf("entradas de entorno derivadas del bloque: %v", candidatas)

	const shaCeros = "0000000000000000000000000000000000000000000000000000000000000000"
	caminos := []struct {
		nombre string
		antes  []string // entorno que arma el caso ANTES de la variable barrida
		porque string
	}{
		{"sin-pin", nil, "una versión sin fila en PINES_RUSTDESK y sin RUSTDESK_SHA256 (no pude medir)"},
		{"no-coincide", []string{"RUSTDESK_SHA256=" + shaCeros},
			"un sha256 con forma válida que NO es el del zip (medí y difiere)"},
	}

	for _, cam := range caminos {
		for _, v := range candidatas {
			for _, valor := range valoresDeEncendido {
				t.Run(cam.nombre+"/"+v+"="+valor, func(t *testing.T) {
					dir, destino, prologo := preparar(t, "9.9.9-sin-fila")
					r := correrArnes(t, dir, []string{prologo, bloque},
						append(append([]string{}, cam.antes...), v+"="+valor))
					if _, err := os.Stat(filepath.Join(destino, "hbbs")); err == nil {
						t.Errorf("con %s=%s y %s, hbbs quedó INSTALADO sin que nadie comparara nada — y "+
							"queda corriendo como unidad systemd de este servidor.\nsalida del guion:\n%s",
							v, valor, cam.porque, r.salida)
					}
					if r.err == nil {
						t.Errorf("con %s=%s y %s el guion salió con código 0: no frenó.\n%s",
							v, valor, cam.porque, r.salida)
					}
				})
			}
		}
	}
}

// TestCuandoNoPudoMedirElGuionFrenaAntesDeMirarElBinario — la guarda de FORMA del sha256.
//
// EL AGUJERO QUE CIERRA: el `if ! [[ "$want" =~ ^[0-9a-f]{64}$ ]]` se puede borrar y las ocho
// subpruebas de la ronda 1 siguen verdes — porque comparar `<html>...` contra el sha real también
// da distinto y también muere. No abre el fail-open; degrada el DIAGNÓSTICO, que es lo único que
// le queda al operador: «no coincide» lo manda a buscar un binario adulterado cuando lo que pasó
// fue que un portal cautivo le contestó el .sha256.
//
// Y NO SE MIRA EL TEXTO DEL MENSAJE, que sería la guarda de siempre satisfecha por un comentario.
// Se mira un DATO derivado de la corrida: el sha256 real del binario descargado. Si el guion
// frenó antes de mirarlo, ese número no puede estar en la salida. Si lo imprime, es porque tomó el
// camino de «no coincide» — o sea, calculó el sha de un binario que nunca debió tocar y le contó
// al operador la historia equivocada.
func TestCuandoNoPudoMedirElGuionFrenaAntesDeMirarElBinario(t *testing.T) {
	const carga = "#!/bin/sh\necho \"musubi 0.0.0-de-prueba\"\n"
	shaCarga := fmt.Sprintf("%x", sha256.Sum256([]byte(carga)))
	const shaCeros = "0000000000000000000000000000000000000000000000000000000000000000"

	casos := []struct {
		nombre string
		modo   string
		binSha string
		// ¿tiene que aparecer el sha del binario en la salida? Sólo cuando la respuesta honesta es
		// «medí las dos cosas y no coinciden».
		nombraElSha bool
		porque      string
	}{
		{"el .sha256 vino vacío", "vacio", "", false,
			"un .sha256 vacío es «no pude medir». Si el mensaje trae el sha del binario es porque se " +
				"comparó contra la nada y se reportó «no coincide»"},
		{"el .sha256 vino con HTML de un portal cautivo", "html", "", false,
			"lo que llegó no tiene forma de sha256: el guion tiene que frenar ANTES de calcular nada " +
				"sobre el binario, y decir que el medio contestó otra cosa"},
		// El sha de este caso NO es el del binario a propósito: si fuera, el mensaje lo nombraría por
		// citar lo que llegó, y la prueba no podría distinguir «lo cité» de «lo medí».
		{"el operador pegó la línea entera del .sha256", "falla", shaCeros + "  musubi-linux-amd64", false,
			"MUSUBI_BIN_SHA256 con el nombre del archivo pegado no es un sha256: es «no pude medir» " +
				"disfrazado de medición, y no se distingue de un binario adulterado si el mensaje dice " +
				"«no coincide»"},
		// Los tres de abajo NO abren el fail-open: los tres frenan igual. Miden que el guion siga
		// sabiendo CUÁL de las dos cosas pasó. Debilitar el regex de forma —`{7,}` en vez de `{64}`,
		// o `.` en vez de `[0-9a-f]`— quedaba verde con las cinco filas de antes, porque comparar
		// cualquier basura contra el sha real también da distinto y también muere. Lo que se pierde es
		// el diagnóstico, que es lo único que le queda al operador.
		{"el operador pegó un sha cortado a 7", "falla", "0000000", false,
			"siete hexadecimales no son un sha256: si el mensaje trae el sha del binario es porque el " +
				"guion aceptó la forma y comparó, y le va a decir «no coincide» a alguien que en " +
				"realidad copió mal"},
		{"el operador pegó un sha de 65", "falla", shaCeros + "0", false,
			"un carácter de más tampoco es un sha256, y el `$` del final es lo único que lo caza"},
		{"lo que llegó tiene 64 caracteres pero no son hexadecimales", "falla",
			"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", false,
			"64 de largo no alcanza: si el guion no mira que sean [0-9a-f], una respuesta de 64 bytes " +
				"de un portal cautivo pasa por medición"},
		{"CONTROL: el .sha256 es válido y NO coincide", "otro", "", true,
			"acá sí se midieron las dos cosas y difieren: el mensaje TIENE que traer el sha real del " +
				"binario, que es el dato con el que el operador averigua qué bajó. Sin este control, la " +
				"prueba pasaría con un guion que nunca imprime nada"},
		{"CONTROL: el operador da un sha válido que no coincide", "falla", shaCeros, true,
			"mismo control por el otro camino de entrada del sha esperado"},
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
			prologo, bloque := arnesDelPaso1(t, dir, destino, stubs)

			entorno := []string{"MODO_SHA=" + c.modo, "SHA_BUENO=" + shaCarga, "CARGA=" + cargaP}
			if c.binSha != "" {
				entorno = append(entorno, "MUSUBI_BIN_SHA256="+c.binSha)
			}
			r := correrArnes(t, dir, []string{prologo, bloque}, entorno)

			if _, err := os.Stat(destino); err == nil {
				t.Fatalf("el binario quedó instalado y no había con qué verificarlo:\n%s", r.salida)
			}
			if r.err == nil {
				t.Fatalf("el guion no frenó:\n%s", r.salida)
			}
			nombra := strings.Contains(r.salida, shaCarga)
			if nombra != c.nombraElSha {
				t.Errorf("%s\n  el mensaje nombra el sha256 real del binario: %v (se esperaba %v)\n"+
					"  sha del binario descargado: %s\n  salida:\n%s",
					c.porque, nombra, c.nombraElSha, shaCarga, r.salida)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// LOS PASOS 5b y 5c: los dos guiones derivados que el instalador deja en el servidor
// ─────────────────────────────────────────────────────────────────────────────────────────────

// guionDerivado describe uno de los dos guiones que install-musubi-brain.sh instala con pin duro.
type guionDerivado struct {
	nombre     string // cómo se llama en el reporte
	archivo    string // el archivo del repo (deploy/<archivo>)
	desde      string // marca de inicio del bloque
	hasta      string
	pin        string   // nombre de la variable del pin
	urlVar     string   // nombre de la variable de la URL de fallback
	destinoVar string   // nombre de la variable del destino
	extras     []string // otras declaraciones que el bloque necesita del guion
	porque     string   // qué se pierde si esto se instala sin verificar
}

func losGuionesDerivados() []guionDerivado {
	return []guionDerivado{
		{
			nombre: "musubi-backup.sh (paso 5b)", archivo: "musubi-backup.sh",
			desde: "# ── 5b. Backup programado", hasta: "# ── 5c. El guion de REDESPLIEGUE",
			pin: "BACKUP_SHA256", urlVar: "BACKUP_SCRIPT_URL", destinoVar: "BACKUP_BIN",
			porque: "lo ejecuta un timer como el usuario del cerebro, con el EnvironmentFile del " +
				"cerebro cargado — o sea, con el token adentro",
		},
		{
			nombre: "redesplegar-cerebro.sh (paso 5c)", archivo: "redesplegar-cerebro.sh",
			desde: "# ── 5c. El guion de REDESPLIEGUE", hasta: "# ── 6. Firewall",
			pin: "REDESPLIEGUE_SHA256", urlVar: "REDESPLIEGUE_SCRIPT_URL", destinoVar: "REDESPLIEGUE_BIN",
			porque: "reemplaza el binario del cerebro y se corre como root: es el peor archivo del " +
				"despliegue para instalar sin verificar, y lo dice el propio instalador",
		},
	}
}

// TestLosGuionesDerivadosNoSeInstalanSinVerificar — el agujero (2), para los dos pasos.
//
// LO QUE LA RONDA 1 DECLARÓ CUBIERTO Y NO LO ESTABA. `TestElPinDelGuionDeBackupEsElVerdadero` y su
// hermano del redespliegue comparan que el pin del instalador coincida con el sha del archivo del
// repo. Eso custodia que el pin no se PUDRA — y nada más. Se le borraron al instalador el `if` de
// comparación y el `die` enteros, dejando el pin declarado con su valor correcto, y el paquete
// entero quedó verde: los dos guiones se instalan con lo que venga.
//
// Acá se corre el bloque de verdad. El archivo bueno es el del repo —cuyo sha ES el pin, y eso lo
// custodian las otras dos pruebas—, y el malo es ese mismo archivo con un byte más.
func TestLosGuionesDerivadosNoSeInstalanSinVerificar(t *testing.T) {
	const rel = "install-musubi-brain.sh"
	guion := leerGuionDeDespliegue(t, rel)

	for _, g := range losGuionesDerivados() {
		t.Run(g.nombre, func(t *testing.T) {
			bueno := leerGuionDeDespliegue(t, g.archivo)
			bloque := bloqueEntreMarcas(t, guion, rel, g.desde, g.hasta)

			casos := []struct {
				nombre  string
				junto   string // contenido del archivo hermano del instalador ("" = no está)
				red     string // contenido que sirve el curl ("" = el curl falla)
				instala bool
				porque  string
			}{
				{"el archivo de al lado es el del repo", bueno, "", true,
					"el guion coincidía con el pin y aun así NO se instaló: el camino verde está roto, " +
						"y con él el valor de todos los rojos de abajo"},
				{"el archivo de al lado tiene un byte de más", bueno + "\n# tocado\n", "", false,
					"el guion NO coincidía con el pin del instalador y se instaló igual"},
				{"no está al lado y la red trae el del repo", "", bueno, true,
					"bajado de main y coincidiendo con el pin, tenía que instalarse"},
				{"no está al lado y la red trae otro", "", "#!/bin/sh\n# esto no es el guion\n", false,
					"lo que bajó de main NO coincidía con el pin —main no tiene branch protection, así " +
						"que eso es exactamente lo que el pin existe para cazar— y se instaló igual"},
				{"no está al lado y la red no contesta", "", "", false,
					"no se pudo conseguir el guion por ningún lado y se instaló algo igual"},
			}

			for _, c := range casos {
				t.Run(c.nombre, func(t *testing.T) {
					dir := t.TempDir()
					stubs := filepath.Join(dir, "stubs")
					aqui := filepath.Join(dir, "aqui")
					for _, d := range []string{stubs, aqui} {
						if err := os.MkdirAll(d, 0o755); err != nil {
							t.Fatal(err)
						}
					}
					escribirStub(t, stubs, "install", stubInstall)
					escribirStub(t, stubs, "systemctl", "exit 0\n")
					if c.red == "" {
						escribirStub(t, stubs, "curl", "exit 22\n")
					} else {
						cuerpo := filepath.Join(dir, "de-la-red")
						if err := os.WriteFile(cuerpo, []byte(c.red), 0o644); err != nil {
							t.Fatal(err)
						}
						escribirStub(t, stubs, "curl", `
destino=""
while (($#)); do case "$1" in -o) destino="$2"; shift 2 ;; *) shift ;; esac; done
[[ -n "$destino" ]] || exit 90
cp `+shQuote(cuerpo)+` "$destino"
`)
					}
					if c.junto != "" {
						if err := os.WriteFile(filepath.Join(aqui, g.archivo), []byte(c.junto), 0o755); err != nil {
							t.Fatal(err)
						}
					}
					destino := filepath.Join(dir, filepath.Base(g.archivo)+"-instalado")

					prologo := strings.Join([]string{
						"#!/usr/bin/env bash",
						"set -euo pipefail",
						"export PATH=" + shQuote(stubs) + `:"$PATH"`,
						funcionesDeSalida(t, guion, rel),
						// El PIN sale del guion TAL CUAL: si alguien lo vacía, el arnés corre con el pin
						// vacío y la prueba lo ve. Escribirlo acá sería probar otro instalador.
						declaracionDelGuion(t, guion, rel, g.pin),
						"AQUI=" + shQuote(aqui),
						"BRAIN_USER=usuario-de-prueba",
						"BRAIN_HOME=" + shQuote(filepath.Join(dir, "brain-home")),
						"BIN=" + shQuote(filepath.Join(dir, "musubi")),
						"ENV_FILE=" + shQuote(filepath.Join(dir, "musubi.env")),
						g.urlVar + "=https://ejemplo.invalido/" + g.archivo,
						g.destinoVar + "=" + shQuote(destino),
						"BACKUP_UNIT=" + shQuote(filepath.Join(dir, "musubi-backup.service")),
						"BACKUP_TIMER=" + shQuote(filepath.Join(dir, "musubi-backup.timer")),
					}, "\n")

					r := correrArnes(t, dir, []string{prologo, bloque}, nil)

					if r.instaloEn(destino) != c.instala {
						t.Errorf("%s\n  %s\n  se instaló: %v (se esperaba %v)\n  el guion salió con: %v\n  salida:\n%s",
							c.porque, g.porque, r.instaloEn(destino), c.instala, r.err, r.salida)
					}
					if c.instala {
						if r.err != nil {
							t.Errorf("el camino verificado tiene que terminar bien y salió con %v\n%s", r.err, r.salida)
						}
						copia, err := os.ReadFile(filepath.Join(r.caja, filepath.Base(destino)))
						if err != nil || string(copia) != c.junto+c.red {
							t.Errorf("se instaló algo distinto de lo que se verificó (%v)", err)
						}
					}
				})
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL INSTALADOR DE PROMETHEUS
// ─────────────────────────────────────────────────────────────────────────────────────────────

// tarDePrometheus arma un .tar.gz con la misma forma que el oficial: <nombre>/prometheus y
// <nombre>/promtool. Así el `tar -xzf` y los dos `install` del guion son los de verdad.
func tarDePrometheus(t *testing.T, nombre string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, b := range []string{"prometheus", "promtool"} {
		cuerpo := []byte("#!/bin/sh\necho " + b + " de prueba\n")
		if err := tw.WriteHeader(&tar.Header{
			Name: nombre + "/" + b, Mode: 0o755, Size: int64(len(cuerpo)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(cuerpo); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestPrometheusNoSeInstalaSinVerificar — el agujero (3).
//
// EL CABO: `deploy/prometheus/install-musubi-prometheus.sh` tenía los dos `die` que importan —el
// que exige que el sha esperado no venga vacío y el que compara esperado contra obtenido— en
// líneas `[ ... ] || die`. Se borraron las dos, dejando el `curl`, el `awk`, el comentario
// «verificamos contra él» y el `ok "Checksum verificado"`, y el paquete entero quedó verde: el
// guion IMPRIME que verificó y no verificó nada. Es el mismo fail-open que el paso 1 del cerebro,
// sobre binarios que quedan como servicio systemd de este mismo servidor.
//
// El caso del `want` VACÍO es el que más importa y el que menos se ve: `sha256sums.txt` bajó, el
// `awk` no encontró la fila —el nombre del asset cambió, o el que contestó fue un portal— y `want`
// queda "". Sin el `die`, comparar "" contra el sha real da distinto... salvo que alguien invierta
// la comparación; y con el `die` borrado se instala igual. Un vacío tiene que significar «no pude
// medir», nunca «medí y está bien».
func TestPrometheusNoSeInstalaSinVerificar(t *testing.T) {
	const rel = "prometheus/install-musubi-prometheus.sh"
	guion := leerGuionDeDespliegue(t, rel)
	bloque := bloqueEntreMarcas(t, guion, rel, "# ── 2. Binarios de Prometheus", "# ── 3. Directorios")

	const version = "2.53.2"
	const arch = "amd64"
	nombre := "prometheus-" + version + ".linux-" + arch
	paquete := tarDePrometheus(t, nombre)
	shaPaquete := fmt.Sprintf("%x", sha256.Sum256(paquete))
	const shaCeros = "0000000000000000000000000000000000000000000000000000000000000000"

	casos := []struct {
		nombre  string
		sumas   string // lo que sirve el curl como sha256sums.txt
		instala bool
		porque  string
	}{
		{"el sha256sums.txt trae el sha correcto",
			shaPaquete + "  " + nombre + ".tar.gz\n", true,
			"el checksum coincidía y aun así NO se instaló: el camino verde está roto, y con él el " +
				"valor de los rojos de abajo"},
		{"el sha256sums.txt trae otro sha",
			shaCeros + "  " + nombre + ".tar.gz\n", false,
			"el paquete NO coincidía con el checksum publicado y prometheus/promtool quedaron " +
				"instalados igual, como servicio de este servidor"},
		{"el sha256sums.txt no tiene la fila de este paquete",
			shaCeros + "  prometheus-9.9.9.linux-amd64.tar.gz\n", false,
			"el sha esperado quedó VACÍO —el awk no encontró la fila— y se instaló igual. Un vacío " +
				"significa «no pude medir», y eso tiene que frenar, no pasar"},
		{"el sha256sums.txt es una página HTML de un portal cautivo",
			"<html><body>Inicia sesion para navegar</body></html>\n", false,
			"lo que llegó no era el archivo de sumas y se instaló igual"},
		{"el sha256sums.txt vino vacío", "", false,
			"el archivo de sumas llegó vacío y se instaló igual"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			stubs := filepath.Join(dir, "stubs")
			if err := os.MkdirAll(stubs, 0o755); err != nil {
				t.Fatal(err)
			}
			paqueteP := filepath.Join(dir, "prom.tar.gz")
			sumasP := filepath.Join(dir, "sha256sums.txt")
			if err := os.WriteFile(paqueteP, paquete, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sumasP, []byte(c.sumas), 0o644); err != nil {
				t.Fatal(err)
			}
			escribirStub(t, stubs, "install", stubInstall)
			escribirStub(t, stubs, "curl", `
destino=""; url=""
while (($#)); do
  case "$1" in -o) destino="$2"; shift 2 ;; -*) shift ;; *) url="$1"; shift ;; esac
done
[[ -n "$destino" ]] || exit 90
case "$url" in
  *sha256sums.txt) cp "$SUMAS" "$destino" ;;
  *.tar.gz)        cp "$PAQUETE" "$destino" ;;
  *) exit 91 ;;
esac
`)
			prologo := strings.Join([]string{
				"#!/usr/bin/env bash",
				"set -euo pipefail",
				"export PATH=" + shQuote(stubs) + `:"$PATH"`,
				funcionesDeSalida(t, guion, rel),
				"PROM_VERSION=" + version,
				"ARCH=" + arch,
			}, "\n")

			r := correrArnes(t, dir, []string{prologo, bloque},
				[]string{"PAQUETE=" + paqueteP, "SUMAS=" + sumasP})

			if r.instaloAlgo() != c.instala {
				t.Errorf("%s\n  se instaló algo: %v (se esperaba %v) → %v\n  el guion salió con: %v\n  salida:\n%s",
					c.porque, r.instaloAlgo(), c.instala, r.instalados, r.err, r.salida)
			}
			if !c.instala && r.err == nil {
				t.Errorf("%s\n  el guion salió con código 0: no frenó\n  salida:\n%s", c.porque, r.salida)
			}
			if c.instala && len(r.instalados) != 2 {
				t.Errorf("el camino verificado tiene que instalar prometheus Y promtool, e instaló %v\n%s",
					r.instalados, r.salida)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL REDESPLIEGUE DEL CEREBRO
// ─────────────────────────────────────────────────────────────────────────────────────────────

// TestElRedespliegueNoAceptaUnShaQueSaleDelBinarioQueVerifica — el agujero (4).
//
// EL CABO: en `deploy/redesplegar-cerebro.sh`, la línea que exige el sha esperado se cambió por
// una que lo ADOPTA del binario que le pasan cuando no se lo dan. La comparación de abajo queda
// vacuamente cierta —el sha del archivo contra el sha del mismo archivo— y el redespliegue sigue
// diciendo «sha256 verificado». Es A111 letra por letra: la comprobación que no puede ponerse roja
// se ve idéntica a la que funciona, y ésa pasó seis redespliegues sin comprobar nada.
//
// El arnés corre el tramo que decide con la declaración de argumentos del propio guion, así que un
// `SHA_ESPERADO="${2:-$(sha256sum "$1")}"` también lo agarraría. Lo único que se deja afuera es la
// exigencia de root (`[[ $EUID -eq 0 ]]`), que no se puede satisfacer en una prueba y no decide
// nada sobre el checksum. La consecuencia que se mide es si el guion SIGUE: si sigue, despliega.
func TestElRedespliegueNoAceptaUnShaQueSaleDelBinarioQueVerifica(t *testing.T) {
	const rel = "redesplegar-cerebro.sh"
	guion := leerGuionDeDespliegue(t, rel)
	argumentos := bloqueEntreMarcas(t, guion, rel, `NUEVO="${1:-}"`, `DESTINO=`)
	bloque := bloqueEntreMarcas(t, guion, rel, `[[ -n "$NUEVO" && -f "$NUEVO" ]]`, "VERSION_NUEVA=")

	const carga = "#!/bin/sh\necho \"musubi 0.0.0-de-prueba\"\n"
	shaCarga := fmt.Sprintf("%x", sha256.Sum256([]byte(carga)))

	casos := []struct {
		nombre string
		sha    string
		sigue  bool
		porque string
	}{
		{"el sha coincide", shaCarga, true,
			"el sha era el correcto y el guion frenó igual: el camino verde está roto y no habría " +
				"forma verificada de redesplegar el cerebro — que es lo que empuja a la copia a mano, " +
				"que es lo que produjo A111"},
		{"no se pasa el sha", "", false,
			"NO se dio el sha esperado y el redespliegue siguió. O falta el `die`, o el guion se lo " +
				"ADOPTÓ del binario que le pasaron: en los dos casos se despliega un binario que nadie " +
				"verificó, sobre el cerebro central, como root"},
		{"el sha es otro", "0000000000000000000000000000000000000000000000000000000000000000", false,
			"el sha no coincidía y el redespliegue siguió"},
		{"el sha es basura", "no-es-un-sha", false,
			"lo que se pasó como sha no tiene forma de sha256 y el redespliegue siguió"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			nuevo := filepath.Join(dir, "musubi-nuevo")
			if err := os.WriteFile(nuevo, []byte(carga), 0o755); err != nil {
				t.Fatal(err)
			}
			marca := filepath.Join(dir, "siguio")
			arnes := []string{
				"#!/usr/bin/env bash",
				"set -uo pipefail",
				funcionesDeSalida(t, guion, rel),
				argumentos,
				"DESTINO=" + shQuote(filepath.Join(dir, "musubi")),
				bloque,
				": > " + shQuote(marca),
			}
			args := []string{nuevo}
			if c.sha != "" {
				args = append(args, c.sha)
			}
			arnesP := filepath.Join(dir, "arnes.sh")
			if err := os.WriteFile(arnesP, []byte(strings.Join(arnes, "\n")+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", append([]string{arnesP}, args...)...)
			var salida bytes.Buffer
			cmd.Stdout, cmd.Stderr = &salida, &salida
			err := cmd.Run()

			_, errMarca := os.Stat(marca)
			siguio := errMarca == nil
			if siguio != c.sigue {
				t.Errorf("%s\n  el guion siguió: %v (se esperaba %v)\n  salió con: %v\n  salida:\n%s",
					c.porque, siguio, c.sigue, err, salida.String())
			}
			if !c.sigue && err == nil {
				t.Errorf("%s\n  y salió con código 0: no frenó\n%s", c.porque, salida.String())
			}
		})
	}
}
