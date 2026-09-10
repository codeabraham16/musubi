package testbudget

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rutaCI es el workflow que gobierna el presupuesto de la suite.
const rutaCI = ".github/workflows/ci.yml"

// EL TECHO NO SE PUEDE VOLVER A TIPEAR EN EL ci.yml.
//
// El defecto de fondo del cabo no fue el número: fue que el número vivía tipeado en dos comandos
// y justificado en prosa al lado, así que envejeció tres veces sin quien lo contradijera. Esta
// guarda cierra el camino de vuelta: cualquier `go test` del ci.yml que lleve `-timeout` tiene
// que tomarlo de $RACE_TIMEOUT, que sale de presupuesto-de-pruebas.env.
//
// Y la excepción, escrita: los benchmarks de escala (bench-scale.yml) tienen su propio techo
// porque miden otra cosa; por eso la guarda se ancla a ci.yml y no a todo .github.
func TestElCiNoTipeaElTecho(t *testing.T) {
	raiz, err := RaizDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(raiz, rutaCI))
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", rutaCI, err)
	}

	// El ancla es un `go test` REAL, no la palabra suelta: las líneas de comentario que hablan
	// de `go test` empiezan con `#` y no son un comando.
	reComando := regexp.MustCompile(`(^|[;&|]\s*)go test\b`)
	// Un literal de duración de Go pegado al flag: `-timeout 20m`, `-timeout=20m`, `-timeout 1h30m`.
	reLiteral := regexp.MustCompile(`-timeout[= ]"?[0-9]+(?:\.[0-9]+)?(?:ns|us|µs|ms|s|m|h)`)

	var conTimeout, tipeados []string
	for _, linea := range strings.Split(string(b), "\n") {
		sinEspacios := strings.TrimSpace(linea)
		if strings.HasPrefix(sinEspacios, "#") {
			continue
		}
		if !reComando.MatchString(sinEspacios) || !strings.Contains(sinEspacios, "-timeout") {
			continue
		}
		conTimeout = append(conTimeout, sinEspacios)
		if reLiteral.MatchString(sinEspacios) {
			tipeados = append(tipeados, sinEspacios)
		}
	}

	// CERO NO PUEDE SIGNIFICAR «NO ENCONTRÉ NADA». Si el ci.yml se renombrara, se reescribiera,
	// o los pasos de test desaparecieran, esta guarda quedaría verde sin haber mirado nada —
	// que es la forma exacta en que este repo perdió siete guardas.
	if len(conTimeout) == 0 {
		t.Fatalf("%s no tiene NI UN `go test` con -timeout: o el workflow cambió de forma y esta "+
			"guarda dejó de mirar lo que dice mirar, o el techo explícito se perdió y la suite "+
			"volvió al default de 10m de Go", rutaCI)
	}
	for _, l := range tipeados {
		t.Errorf("%s tipea el techo en vez de leerlo de %s:\n    %s\n"+
			"    Usá -timeout \"$RACE_TIMEOUT\" (o ${{ env.RACE_TIMEOUT }}), que sale del paso "+
			"«Presupuesto de pruebas (política)».", rutaCI, NombreArchivoPolitica, l)
	}
}

// Los pasos que publican RACE_TIMEOUT tienen que leer el archivo de política. Sin esto, alguien
// podría poner `echo RACE_TIMEOUT=20m >> $GITHUB_ENV` y la variable existiría igual, con el
// número tipeado de vuelta un nivel más adentro.
func TestElCiLeeElArchivoDePolitica(t *testing.T) {
	raiz, err := RaizDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(raiz, rutaCI))
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", rutaCI, err)
	}
	texto := string(b)

	// Cuántos jobs publican la variable, y cuántos la publican DESPUÉS de leer el archivo.
	publican := strings.Count(texto, `>> "$GITHUB_ENV"`)
	if publican == 0 {
		t.Fatalf("%s no publica RACE_TIMEOUT en $GITHUB_ENV: los `go test` que la usan la verían "+
			"vacía y `go test` caería en su default de 10m", rutaCI)
	}
	leen := strings.Count(texto, "source ./"+NombreArchivoPolitica)
	if leen != publican {
		t.Errorf("%s publica %d variable(s) en $GITHUB_ENV pero sólo %d salen de `source ./%s`: "+
			"alguna se está tipeando a mano, que es el defecto que esto cierra",
			rutaCI, publican, leen, NombreArchivoPolitica)
	}

	// Y el archivo tiene que existir con ese nombre exacto, si no el `source` muere en CI.
	if _, err := os.Stat(filepath.Join(raiz, NombreArchivoPolitica)); err != nil {
		t.Fatalf("%s hace `source ./%s` y el archivo no está: %v", rutaCI, NombreArchivoPolitica, err)
	}
}
