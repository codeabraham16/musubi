package selfupdate

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// LA CADENA DE AUTO-UPDATE ESTUVO CONSTRUIDA Y APAGADA. Medido el 2026-09-10: `release.yml` no
// empotraba la clave pública, así que `clavePublicaDeRelease()` devolvía nil en TODO binario
// publicado y `musubi update` fallaba con «este binario no trae una clave pública de release
// válida», pasara lo que pasara con el manifiesto. Y ninguno de los seis releases revisados
// (v0.131.0 … v0.103.0) tenía manifiesto: nunca se firmó ninguno.
//
// Firma, manifiesto, ShaDeAsset y VerificarFirma existían, con tests, y el interruptor estaba en
// off. Estos dos tests son el interruptor: el primero exige que el release empotre la clave, el
// segundo que un build cualquiera NO la traiga.

var inyectaLaClave = regexp.MustCompile(`-X\s+main\.clavePublicaDeReleaseHex=([0-9a-fA-F]+)`)

func TestElReleaseEmpotraLaClavePublica(t *testing.T) {
	ruta := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	yml, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no pude leer %s: %v", ruta, err)
	}

	m := inyectaLaClave.FindSubmatch(yml)
	if m == nil {
		t.Fatal("release.yml NO empotra main.clavePublicaDeReleaseHex en sus -ldflags. Sin eso, " +
			"todo binario publicado devuelve nil en clavePublicaDeRelease() y `musubi update` se " +
			"niega a instalar cualquier release, firmado o no: la cadena entera queda decorativa")
	}
	// 64 hex = 32 bytes = una clave ed25519. Un valor corto o con basura decodifica a nil por el
	// mismo camino que no tener clave, y el síntoma sería idéntico: «no trae una clave válida».
	if clave := string(m[1]); len(clave) != 64 {
		t.Errorf("la clave empotrada mide %d caracteres hex y una ed25519 son 64: %q. "+
			"clavePublicaDeRelease() la va a descartar y el release quedaría sin poder verificarse",
			len(clave), clave)
	}
}

// Y LA OTRA MITAD DEL INVARIANTE, que no es simetría: el default tiene que seguir VACÍO. Si
// alguien "arregla" lo de arriba poniendo la clave como valor por omisión en clave_release.go, un
// `go build` cualquiera —el de un dev, el de una prueba— pasaría a decir que puede verificar
// releases. El comentario de ese archivo lo declara y hasta hoy nada lo sostenía.
func TestUnBuildCualquieraNoTraeClave(t *testing.T) {
	ruta := filepath.Join("..", "..", "cmd", "musubi", "clave_release.go")
	src, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no pude leer %s: %v", ruta, err)
	}
	const vacia = `var clavePublicaDeReleaseHex = ""`
	if !regexp.MustCompile(regexp.QuoteMeta(vacia)).Match(src) {
		t.Errorf("cmd/musubi/clave_release.go ya no declara %s. El default VACÍO es lo que hace "+
			"que un build sin firmar no pueda auto-actualizarse; con una clave por omisión, "+
			"cualquier binario compilado a mano afirmaría que verifica releases", vacia)
	}
}
