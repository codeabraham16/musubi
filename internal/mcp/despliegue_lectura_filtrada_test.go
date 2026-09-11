package mcp

import (
	"os"
	"strings"
	"testing"
)

// NINGUNA GUARDA PUEDE LEER UN ARCHIVO DE `deploy/` SIN PASARLO POR EL FILTRO DE COMENTARIOS.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA CLASE, Y POR QUÉ HIZO FALTA ESTO Y NO SEIS ARREGLOS
//
// El defecto dominante de las guardas de este repo no es que falte una guarda: es que la guarda
// EXISTE, está en verde, y lo que la satisface es un comentario. Se midió una vez: de 46 guardas
// que leen archivos no-Go, SIETE quedaban verdes sobre su propio sabotaje. Y pasó tres veces en
// un solo día con los guiones de Windows — el comentario que EXPLICA el arreglo satisfacía la
// guarda DEL arreglo, así que borrar el arreglo y dejar su documentación salía verde.
//
// EL PRIMER INTENTO FUE UN HELPER OPCIONAL (`codigoDe`) Y NO ALCANZÓ, por el motivo más
// aburrido: un helper que hay que acordarse de llamar se olvida. En el archivo que lo inventó,
// CINCO de sus siete aserciones seguían leyendo el texto crudo. Y peor: 31 guardas de este
// paquete leían `deploy/` con `os.ReadFile` directo, ni siquiera pasando por `leerDeploy`. Una de
// ellas era el candado de `%~dp0` de `cambiar-agente.cmd`, que se satisfacía con su propia línea
// `REM` — y ninguna guarda del `.cmd` filtraba `REM`, porque el filtro que existía era el de
// bash.
//
// ASÍ QUE EL DEFAULT SE INVIRTIÓ: leer devuelve CÓDIGO, y ver comentarios hay que pedirlo por
// nombre (`leerDeployCrudo`, o `os.ReadFile` con un motivo escrito). Esta prueba es lo que
// sostiene la inversión. Sin ella la lista vuelve a crecer sola, y nadie se entera hasta que un
// sabotaje sale verde.
//
// ESTA PRUEBA ES EL «INVENTARIO» QUE FALTABA. No existía la lista de guardas que leen un archivo
// no-Go sin filtrar comentarios, así que cada hallazgo era suelto y la clase nunca se cerraba.
// Producir la lista es un comando; mantenerla al día es esta prueba.
// ────────────────────────────────────────────────────────────────────────────────────────────

// marcaDeCrudoAProposito es el permiso explícito para leer un archivo de deploy/ sin filtrar.
//
// VA EN LA MISMA LÍNEA Y CON EL MOTIVO ESCRITO, no en una lista aparte. Una lista de archivos
// exentos exime al ARCHIVO entero: la próxima lectura cruda de ese mismo archivo entraría sin
// que nadie la mire, que es el agujero que esta prueba existe para tapar. El permiso es por
// lectura, y quien lo pone tiene que decir por qué en el lugar donde alguien lo va a leer.
const marcaDeCrudoAProposito = "// crudo:"

// Partidos a propósito: ver el comentario de adentro del bucle.
const lecturaCruda = "os." + "ReadFile("
const carpetaVigilada = "dep" + "loy"

func TestNingunaGuardaLeeUnArchivoDeDespliegueSinFiltrarComentarios(t *testing.T) {
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude listar el paquete: %v", err)
	}

	revisados, conMotivo := 0, 0
	for _, e := range entradas {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		crudo, err := os.ReadFile(nombre)
		if err != nil {
			t.Fatalf("no pude leer %s: %v", nombre, err)
		}
		revisados++
		for n, linea := range strings.Split(string(crudo), "\n") {
			desnuda := strings.TrimSpace(linea)
			if strings.HasPrefix(desnuda, "//") {
				continue
			}
			// LOS DOS LITERALES NO PUEDEN COMPARTIR LÍNEA, y no es un capricho de estilo: la
			// primera versión los tenía juntos y esta guarda SE DETECTÓ A SÍ MISMA. Exceptuar el
			// archivo propio habría sido lo cómodo y habría dejado ciega a la guarda sobre su
			// propio archivo — que es la forma exacta del defecto que persigue.
			if !strings.Contains(linea, lecturaCruda) {
				continue
			}
			if !strings.Contains(linea, carpetaVigilada) {
				continue
			}
			if strings.Contains(linea, marcaDeCrudoAProposito) {
				conMotivo++
				continue
			}
			t.Errorf("%s:%d lee un archivo de deploy/ con `os.ReadFile` directo.\n"+
				"  Eso se saltea el blanqueo de comentarios, y entonces la guarda la puede satisfacer\n"+
				"  una línea de prosa: el sabotaje «borrar el arreglo y dejar el comentario que lo\n"+
				"  explica» sale VERDE. Ya pasó siete veces en este repo.\n"+
				"  Usá `leerArchivoDeDespliegue` —misma firma, es sólo cambiar el nombre— o, si de\n"+
				"  verdad necesitás el texto entero, `leerDeployCrudo` con el motivo escrito al lado\n"+
				"  y una entrada en `permitidasSinFiltrar`.\n"+
				"  Línea: %s", nombre, n+1, desnuda)
		}
	}

	// LA OTRA DIRECCIÓN. Sin esto, la guarda la satisface poner la marca en todas partes: el
	// permiso dejaría de ser una excepción y volvería a ser el default, con más ceremonia. Hoy
	// son dos —los sha256 de los guiones que el instalador verifica, que por definición son del
	// archivo ENTERO— y si ese número crece hay que ir a mirar cada motivo nuevo.
	if conMotivo > 4 {
		t.Errorf("hay %d lecturas crudas con motivo escrito, y eran 2: el permiso explícito se está "+
			"volviendo la regla. Andá a leer los motivos nuevos: si alguno no custodia una propiedad "+
			"del TEXTO ENTERO, la lectura tiene que ir filtrada.", conMotivo)
	}

	// EL CONTROL. Sin esto, el día que estas guardas se muden de paquete la prueba pasaría en
	// verde sin haber abierto un solo archivo, y ese verde diría «no hay problema» en vez de «no
	// miré» — que es exactamente el defecto que esta prueba persigue, una vuelta más adentro.
	if revisados < 40 {
		t.Fatalf("sólo se revisaron %d archivos de prueba del paquete, y hay bastantes más: "+
			"probablemente cambió dónde viven y esta guarda está en verde sin mirar nada", revisados)
	}
}
