package mcp

// EL NOMBRE CON EL QUE EL ALMACÉN GUARDA EL DISPARO DE UNA POLÍTICA, PREGUNTADO AL ALMACÉN.
//
// (Revisión de A131·T4.) Dos guardas de T4 comparaban lo que queda en fleet_policy_state contra un
// nombre que la prueba suponía: la poda horaria, contra el nombre de s.politicas; la carga del
// arranque, contra una clave armada con el nombre recortado de la fila. Las dos fijaban DÓNDE se
// recorta el `name:` —hoy, en ConfigurarFlota— y no lo que se observa. Un arreglo distinto y correcto
// (dejar el nombre crudo en la política y recortar en cada lector: la carga, la poda y la
// deduplicación) las ponía en rojo con mensajes falsos: «la poda horaria se llevó el cooldown
// persistido de " revivir-db "» con la fila `revivir-db` viva en la tabla.
//
// No se arregla copiando en la prueba la regla del almacén: un TrimSpace escrito a mano es un derivado
// que envejece el día que el almacén normalice de otra forma. Se le PREGUNTA al almacén, marcando un
// disparo y mirando qué nombre aparece.
//
// Y LOS BORDES SON BLANCOS MEZCLADOS, NO SÓLO EL ESPACIO (bordesDeBlancos, al final del archivo).

import (
	"slices"
	"sort"
	"testing"
	"time"
	"unicode"

	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// politicasGuardadas son los nombres de política que tienen alguna fila en fleet_policy_state.
func politicasGuardadas(t *testing.T, e memory.StorageBackend) map[string]bool {
	t.Helper()
	cds, err := e.CooldownsDePoliticas()
	if err != nil {
		t.Fatalf("CooldownsDePoliticas: %v", err)
	}
	out := make(map[string]bool, len(cds))
	for politica := range cds {
		out[politica] = true
	}
	return out
}

// marcarYVerComoSeGuarda marca un disparo de `politica` y devuelve el nombre con el que el almacén lo
// guardó: el ÚNICO nombre de política que aparece en fleet_policy_state después de marcar y no estaba
// antes. Si no aparece ninguno, el almacén ya tenía filas con ese nombre —dos políticas que no
// distingue—, y eso se denuncia en vez de devolver un vacío que el llamador leería como un nombre.
func marcarYVerComoSeGuarda(t *testing.T, e memory.StorageBackend, politica, deviceID, alcance string, cuando time.Time) string {
	t.Helper()
	antes := politicasGuardadas(t, e)
	if err := e.MarcarDisparoDePolitica(politica, deviceID, alcance, cuando); err != nil {
		t.Fatalf("marcar el disparo de %q: %v", politica, err)
	}
	var nuevos []string
	for nombre := range politicasGuardadas(t, e) {
		if !antes[nombre] {
			nuevos = append(nuevos, nombre)
		}
	}
	sort.Strings(nuevos)
	if len(nuevos) != 1 {
		t.Fatalf("marcar el disparo de %q dejó %d nombre/s de política nuevo/s en fleet_policy_state (%q) y tenía "+
			"que ser uno: con cero, el almacén lo guardó con el nombre de otra que ya tenía filas —dos políticas que "+
			"no distingue—; con más, la prueba no sabe cuál es el suyo", politica, len(nuevos), nuevos)
	}
	return nuevos[0]
}

// nombresGuardados devuelve, en el mismo orden, el nombre con el que el almacén guarda el disparo de
// cada política. Se mide en un almacén APARTE, vacío y del que no lee nadie, para no dejarle filas de
// más al de la prueba.
func nombresGuardados(t *testing.T, pols []fleet.Politica) []string {
	t.Helper()
	espejo, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer espejo.Close()
	cuando := time.Now().UTC().Truncate(time.Second)
	out := make([]string, 0, len(pols))
	for _, p := range pols {
		out = append(out, marcarYVerComoSeGuarda(t, espejo, p.Nombre, "espejo", p.Servicio, cuando))
	}
	return out
}

// blancosDeUnicode son todas las runas que unicode.IsSpace da por blanco, en orden de código: las que
// recorta strings.TrimSpace. Se derivan de unicode y no se escriben: una lista a mano se queda con los
// blancos que uno recuerda, y el borde vuelve a quedar clavado en ésos.
func blancosDeUnicode() []rune {
	var out []rune
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) {
			out = append(out, r)
		}
	}
	return out
}

// bordesDeBlancos devuelve los dos bordes con que las guardas de T4 escriben un `name:` —y un
// `service:`— «con bordes»: blancos MEZCLADOS, no uno solo.
//
// ASÍ ESTABA, Y POR QUÉ NO ALCANZABA (revisión de A131·T4). Los bordes eran sólo el espacio ASCII
// (`" revivir-db "`), y construir la política con `strings.Trim(pc.Name, " ")` en vez de TrimSpace
// dejaba internal/mcp en verde salvo el censo: el eje del borde estaba clavado en el espacio. El
// almacén recorta con TrimSpace, así que un `name:` con un tab o un salto de línea en el borde habría
// vuelto a perder el cooldown en cada reinicio y en cada poda sin que nada se pusiera rojo.
//
// Cada borde lleva TODOS los blancos de unicode: por fuera un espacio ASCII y, adentro, los demás —en
// orden de código a la izquierda y al revés a la derecha—. EL ESPACIO VA POR FUERA A PROPÓSITO: un
// recorte que sólo conoce el espacio se come esa capa y se frena en el primer blanco que no lo es (el
// tab a la izquierda), así que deja un nombre que no es ni el crudo ni el recortado, y el rojo lo dice
// con el nombre que quedó. Con otro blanco por fuera no recortaría nada, y su rojo sería idéntico al
// de no recortar: el arnés los contaría como un mismo sabotaje.
func bordesDeBlancos(t *testing.T) (izquierda, derecha string) {
	t.Helper()
	var adentro []rune
	for _, r := range blancosDeUnicode() {
		if r != ' ' {
			adentro = append(adentro, r)
		}
	}
	// PISO: blancos de control ASCII y blancos fuera de ASCII. Sin las dos familias el borde vuelve a
	// medir una sola, que es el eje que esta función vino a abrir.
	fueraDeASCII := slices.ContainsFunc(adentro, func(r rune) bool { return r > unicode.MaxASCII })
	if !slices.Contains(adentro, '\t') || !slices.Contains(adentro, '\n') || !fueraDeASCII {
		t.Fatalf("PISO: el borde de blancos quedó en %q: le falta el tab, el salto de línea o un blanco fuera de ASCII", string(adentro))
	}
	izquierda = " " + string(adentro)
	slices.Reverse(adentro)
	return izquierda, string(adentro) + " "
}
