package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EN UNA CADENA DE POWERSHELL, `\"` NO ESCAPA NADA — Y EL MISMO PAR ES CORRECTO EN BASH.
//
// LO QUE PASÓ, MEDIDO EN `davantis-1` EL 2026-09-10. `matar-zombis-agente.sh` llevaba esto adentro
// de un bloque de PowerShell:
//
//	"... (schtasks /run /tn \"Musubi Agente de Flota\"). Procesos: " + $detalle
//
// En una cadena de PowerShell con comillas dobles el carácter de escape es el BACKTICK, no la
// barra invertida. Así que esa comilla CIERRA la cadena y lo que sigue queda suelto:
//
//   - FullyQualifiedErrorId : UnexpectedToken) [], ParentContainsErrorRecordException
//     ✗ no se mató nada
//
// Y LO QUE LO HACE PEOR ES DÓNDE ESTABA: en la rama `$nuevos.Count -eq 0`, que NO SE EJECUTÓ —la
// máquina sí tenía el proceso nuevo—. PowerShell PARSEA EL BLOQUE ENTERO antes de correr una sola
// línea, así que un error de sintaxis en una rama muerta se llevó puesta la rama viva. El paso 1
// había identificado el zombi correctamente; el paso 2 murió sin ejecutar nada. Un bug en un
// camino que nadie toma no es inofensivo cuando el lenguaje parsea antes de ejecutar.
//
// POR QUÉ SE COLÓ, que es lo que hace falta para que no vuelva: el MISMO par de caracteres es
// CORRECTO en las cadenas de bash de ese mismo archivo —el JSON de `musubi_fleet_log`, los `echo`
// del final— y las dos clases de cadena conviven en el mismo renglón. No es distracción: es que
// `\"` significa cosas opuestas según en qué lenguaje esté el texto, y nada lo señalaba.
//
// CÓMO SE MIDE, Y POR QUÉ NO ES UN GREP DEL ARCHIVO. Un `grep '\\"'` sobre el .sh se pone rojo
// sobre los `echo` de bash, que están BIEN. Hay que mirar SÓLO el PowerShell, así que se extraen
// los bloques: son las cadenas de bash con comillas simples que arrancan justo después de un `"'`
// —el cierre de `"$RESOLVER..."` pegado a la apertura del bloque— y terminan en la próxima comilla
// simple, porque adentro no puede haber ninguna sin cerrar la cadena de bash.
//
// MI PRIMER ESCÁNER DIO VERDE SOBRE EL ARCHIVO CON EL BUG, y queda escrito porque es la lección.
// Seguía el estado de comillas de bash carácter por carácter, y el `$( ... )` anidado se lo
// desordenaba: al llegar a `ps1 "` daba por cerrada la comilla de afuera y terminaba leyendo el
// bloque como si fuera una cadena DOBLE, donde `\"` sí es el escape correcto. Lo cacé corriéndolo
// contra una copia del archivo roto ANTES de creerle al verde.
//
// SABOTAJES CORRIDOS:
//   - ROJO: la copia previa al arreglo. Encuentra el `\"` y lo ubica en la línea 121.
//   - ROJO: vaciar la lista de bloques extraídos (el control de abajo). Un verde sobre cero
//     bloques diría «no hay problema» cuando lo que hubo fue «no miré nada».
func TestNingunBloqueDePowerShellEscapaComillasConBarra(t *testing.T) {
	dir := filepath.Join("..", "..", "deploy")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("no pude leer %s: %v", dir, err)
	}

	bloques := 0
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		ruta := filepath.Join(dir, e.Name())
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatalf("no pude leer %s: %v", ruta, err)
		}
		src := string(crudo)
		for i := 0; ; {
			j := strings.Index(src[i:], `"'`)
			if j < 0 {
				break
			}
			ini := i + j + 2
			fin := strings.IndexByte(src[ini:], '\'')
			if fin < 0 {
				break
			}
			bloque := src[ini : ini+fin]
			i = ini + fin + 1
			bloques++
			k := strings.Index(bloque, `\"`)
			if k < 0 {
				continue
			}
			linea := 1 + strings.Count(src[:ini+k], "\n")
			t.Errorf("%s:%d — un bloque de PowerShell escapa una comilla con BARRA (`\\\"`).\n\n"+
				"  …%s…\n\n"+
				"En una cadena de PowerShell con comillas dobles el escape es el BACKTICK; la barra no "+
				"escapa nada, así que esa comilla CIERRA la cadena y el resto queda suelto. El bloque "+
				"entero deja de parsear —PowerShell parsea antes de ejecutar—, así que también se "+
				"rompen las ramas que sí se iban a correr.\n"+
				"Para una comilla literal adentro de la cadena, DOBLALA: `\"\"`.\n"+
				"Ojo: `\\\"` SÍ es correcto en las cadenas de bash de este mismo archivo (los `echo`, "+
				"el JSON de `musubi_fleet_log`), y por eso esta guarda mira sólo los bloques de "+
				"PowerShell y no el archivo entero.",
				e.Name(), linea, recorteDelBloquePS(bloque, k))
		}
	}

	// EL CONTROL. Si la extracción deja de encontrar bloques —porque cambia cómo se arma la llamada
	// a `ps1`, por ejemplo— esta prueba pasaría en verde sin haber mirado una sola línea de
	// PowerShell, y ese verde diría «no hay problema» en vez de «no medí nada».
	if bloques < 10 {
		t.Fatalf("sólo se extrajeron %d bloques de PowerShell de deploy/*.sh, y hay bastantes más. "+
			"Probablemente cambió cómo se arman las llamadas a `ps1` y la extracción dejó de "+
			"encontrarlos: esta guarda estaría en verde sin mirar nada", bloques)
	}
}

func recorteDelBloquePS(s string, en int) string {
	ini := en - 55
	if ini < 0 {
		ini = 0
	}
	fin := en + 25
	if fin > len(s) {
		fin = len(s)
	}
	return strings.ReplaceAll(s[ini:fin], "\n", "⏎")
}
