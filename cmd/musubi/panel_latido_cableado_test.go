package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// EL PANEL CABLEA EL LATIDO: LA HORA DEL SONDEO ES SU `at`, Y EL PIE LO DICE latido().
//
// TestElLatidoAguantaLaBajadaEspaciadaYSeApagaConUnCorte recorre assets/src/latido.mjs en node y
// no mira quién lo llama: eso lo mira ésta. Si anotarEvento vuelve a estampar el sondeo a su
// LLEGADA, o tictac deja de preguntarle a latido(), todo sigue en verde (node --test, la prueba en
// node y el job `panel`, que sólo compara el bundle con el fuente) y el backlog que el relay manda
// al abrir la página vuelve a prender la lámpara con sondeos de hace horas, ahora por diez minutos.
//
// Mira el cableado en los DOS lugares donde vive: el fuente (assets/src/dashboard.mjs), que es lo
// que tocan los sabotajes, y el bundle EMBEBIDO, que es lo que se sirve. En el bundle los nombres
// están minificados y cambian con cada reconstrucción (ver el ancla de
// TestElBundleWebGLNoSabeNadaDeLaFlota), así que ahí se busca la FORMA de la llamada, no el nombre.
//
// Sabotaje: estampar el sondeo a su llegada, la conducta vieja: el backlog de hace 20 min prende la
// lámpara al abrir la página.
// arnes: archivo="cmd/musubi/assets/src/dashboard.mjs"
// arnes: de="VIVO.ultimoSondeo=anotarSondeo(VIVO.ultimoSondeo, e.at, Date.now());"
// arnes: a="VIVO.ultimoSondeo=Date.now();"
//
// Sabotaje: que el pie deje de preguntarle a latido() y vuelva a una ventana propia de un minuto.
// arnes: archivo="cmd/musubi/assets/src/dashboard.mjs"
// arnes: de="const l=latido(VIVO.ultimoSondeo, Date.now());"
// arnes: a="const l={vive:Date.now()-VIVO.ultimoSondeo<60000, texto:'sin sondeo'};"
func TestElPanelCableaElLatidoConLaHoraDelEvento(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("assets", "src", "dashboard.mjs"))
	if err != nil {
		t.Fatalf("no se pudo leer el fuente del panel: %v", err)
	}
	fuente := string(src)

	enFuente := []struct {
		que string
		re  *regexp.Regexp
	}{
		{"anotarEvento le pasa a anotarSondeo el `at` del sondeo y su llegada",
			regexp.MustCompile(`e\.kind\s*===?\s*'sondeo'\s*\)\s*\{\s*VIVO\.ultimoSondeo\s*=\s*anotarSondeo\(\s*VIVO\.ultimoSondeo\s*,\s*e\.at\s*,\s*Date\.now\(\)\s*\)`)},
		{"tictac le pregunta a latido() por la marca del último sondeo",
			regexp.MustCompile(`const\s+l\s*=\s*latido\(\s*VIVO\.ultimoSondeo\s*,\s*Date\.now\(\)\s*\)`)},
		{"la lámpara y el texto del pie salen de lo que dijo latido()",
			regexp.MustCompile(`classList\.toggle\(\s*'vive'\s*,\s*l\.vive\s*\)[\s\S]{0,160}?textContent\s*=\s*l\.texto`)},
	}
	for _, c := range enFuente {
		if n := len(c.re.FindAllStringIndex(fuente, -1)); n != 1 {
			t.Errorf("dashboard.mjs: %s — la forma aparece %d veces y tiene que aparecer una. Sin esto el latido "+
				"que recorre la prueba en node no es el que ve la persona", c.que, n)
		}
	}
	if n := strings.Count(fuente, "VIVO.ultimoSondeo="); n != 1 {
		t.Errorf("dashboard.mjs escribe la marca del último sondeo en %d lugares: tiene que ser uno, el que pasa por "+
			"anotarSondeo (hora del `at`, recortada a la llegada, sin retroceder)", n)
	}

	b := string(assetsFS(t, "assets/dashboard.bundle.js"))
	enBundle := []struct {
		que string
		re  *regexp.Regexp
	}{
		{"la marca del sondeo se anota con su `at` y su llegada",
			regexp.MustCompile(`\.ultimoSondeo=[\w$]+\([\w$]+\.ultimoSondeo,[\w$]+\.at,Date\.now\(\)\)`)},
		{"el pie le pregunta a latido() y pinta su veredicto",
			regexp.MustCompile(`=[\w$]+\([\w$]+\.ultimoSondeo,Date\.now\(\)\);[\w$]+\.classList\.toggle\("vive",[\w$]+\.vive\)`)},
		{"latido() está en el bundle (el tree-shaking lo saca si nadie lo llama)",
			regexp.MustCompile(`"sin sondeo desde hace "`)},
	}
	for _, c := range enBundle {
		if n := len(c.re.FindAllStringIndex(b, -1)); n != 1 {
			t.Errorf("bundle embebido: %s — la forma aparece %d veces y tiene que aparecer una. Lo que se sirve no "+
				"es lo que la prueba del latido recorre", c.que, n)
		}
	}
	if n := strings.Count(b, ".ultimoSondeo="); n != 1 {
		t.Errorf("el bundle embebido escribe la marca del último sondeo en %d lugares: tiene que ser uno", n)
	}
}
