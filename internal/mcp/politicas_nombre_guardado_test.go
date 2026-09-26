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

import (
	"sort"
	"testing"
	"time"

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
