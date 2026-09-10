package selfupdate

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// El firmador de releases (deploy/firmar-release.sh) lleva la lista blanca de qué archivo del
// directorio es un asset y cuál no. Esa lista y AssetName tienen que nombrar EXACTAMENTE lo mismo,
// y hasta el 2026-09-10 no lo hacían: el guion usaba el patrón `^musubi(-[a-z0-9]+)+(\.exe)?$`,
// sensible a mayúsculas y con un guion obligatorio, así que `Musubi.exe` y `Musubi-arm64.exe`
// quedaban afuera. El manifiesto salía sin los dos binarios de Windows y `musubi update` en
// Windows se negaba a instalar un release perfectamente firmado, porque un asset ausente del
// manifiesto es un ERROR y no un «seguí sin verificar» (ver ShaDeAsset).
//
// EL DEFECTO ES INVISIBLE DESDE CADA LADO POR SEPARADO, y por eso duró: el guion no falla —firma
// lo que casa y calla lo que no— y AssetName tampoco, porque devuelve el nombre correcto. Sólo
// aparece CRUZÁNDOLOS, que es lo único que hace este test.

var plataformasSoportadas = []struct{ goos, goarch string }{
	{"windows", "amd64"}, {"windows", "arm64"},
	{"linux", "amd64"}, {"linux", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
}

func TestElFirmadorCubreLosAssetsReales(t *testing.T) {
	guion, err := os.ReadFile("../../deploy/firmar-release.sh")
	if err != nil {
		t.Fatalf("no pude leer deploy/firmar-release.sh: %v", err)
	}
	enElGuion := assetsDelGuion(t, string(guion))

	for _, p := range plataformasSoportadas {
		nombre, err := AssetName(p.goos, p.goarch)
		if err != nil {
			t.Fatalf("AssetName(%s, %s) devolvió error: %v", p.goos, p.goarch, err)
		}
		if !enElGuion[nombre] {
			t.Errorf("%s/%s descarga el asset %q y deploy/firmar-release.sh NO lo incluye en su "+
				"lista blanca: quedaría fuera del manifiesto, y `musubi update` se negaría a "+
				"instalar en esa plataforma un release que igual está bien firmado",
				p.goos, p.goarch, nombre)
		}
		delete(enElGuion, nombre)
	}

	// Y al revés, que es la otra mitad del invariante: nada de más. Un nombre en el guion que
	// ninguna plataforma pide es un archivo que firmamos y publicamos sin que nadie lo vaya a
	// descargar — o el resto de un rename que se hizo a medias.
	if len(enElGuion) > 0 {
		sobran := make([]string, 0, len(enElGuion))
		for n := range enElGuion {
			sobran = append(sobran, n)
		}
		sort.Strings(sobran)
		t.Errorf("deploy/firmar-release.sh firmaría %v, que AssetName no pide para ninguna "+
			"plataforma soportada", sobran)
	}
}

var (
	bloqueDeAssets  = regexp.MustCompile(`(?s)\nASSETS\s*=\s*\{(.*?)\}`)
	literalConComas = regexp.MustCompile(`"([^"]+)"`)
)

// assetsDelGuion saca la lista blanca del guion. Falla RUIDOSO si no la encuentra: si alguien
// renombra el bloque, lo que corresponde es arreglar este test, no que se ponga verde sobre un
// conjunto vacío y siga afirmando que vigila algo. Es la misma trampa del canario que pasó siete
// semanas en rojo sin que nadie leyera por qué.
func assetsDelGuion(t *testing.T, guion string) map[string]bool {
	t.Helper()
	m := bloqueDeAssets.FindStringSubmatch(guion)
	if m == nil {
		t.Fatal("no encontré el bloque `ASSETS = {…}` en deploy/firmar-release.sh: si se renombró " +
			"o se volvió a un patrón, este test dejó de vigilar nada y hay que arreglarlo")
	}
	out := map[string]bool{}
	for _, c := range literalConComas.FindAllStringSubmatch(m[1], -1) {
		out[c[1]] = true
	}
	if len(out) == 0 {
		t.Fatal("el bloque `ASSETS` del firmador está vacío: no hay lista blanca que verificar")
	}
	return out
}
