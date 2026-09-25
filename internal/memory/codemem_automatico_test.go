package memory

import (
	"context"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// codemem_automatico_test.go custodia las reglas del escritor de gists AUTOMÁTICOS: nunca pisa un
// gist de agente, es idempotente sin congelarse, retira sólo lo suyo, y no se hace pasar por el
// agente ante los que cuentan «se guardó algo».

func autoGist(path, texto, fp string) CodeMemory {
	g := PrefijoGistAutomatico + texto
	return CodeMemory{Path: path, Gist: g, Symbols: "Foo L3", Fingerprint: fp, Tokens: EstimateTokens(g)}
}

func generacion(t *testing.T, e *DbEngine) int64 {
	t.Helper()
	g, err := leerGeneracionDelGrafo(context.Background(), e.db)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func filasDelPath(t *testing.T, e *DbEngine, path string) map[string]string {
	t.Helper()
	rows, err := e.db.Query(`SELECT project_id, gist FROM code_memory WHERE path = ?`, path)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var pid, g string
		if err := rows.Scan(&pid, &g); err != nil {
			t.Fatal(err)
		}
		out[pid] = g
	}
	return out
}

// Sabotaje: acotar al proyecto de origen la búsqueda del gist de agente. El gist de agente que
// vive bajo project_id vacío y 'Musubi' deja de verse y nace la fila automática al lado.
// arnes: archivo="internal/memory/codemem.go"
// arnes: de="`SELECT COUNT(*) FROM code_memory WHERE path = ? AND substr(gist, 1, length(?)) != ?`,\n\t\t\tcm.Path, PrefijoGistAutomatico, PrefijoGistAutomatico,"
// arnes: a="`SELECT COUNT(*) FROM code_memory WHERE path = ? AND project_id = ? AND substr(gist, 1, length(?)) != ?`,\n\t\t\tcm.Path, projectID, PrefijoGistAutomatico, PrefijoGistAutomatico,"
func TestElGistAutomaticoNuncaPisaUnoDeAgente(t *testing.T) {
	e := newTestEngine(t)
	for _, pid := range []string{"", "Musubi"} {
		if err := e.SaveCodeMemoryFrom(pid, CodeMemory{Path: "pkg/a.go", Gist: "texto del agente " + pid, Fingerprint: "vieja"}); err != nil {
			t.Fatal(err)
		}
	}
	// Con project_id '' el engine estampa el suyo: se reescribe a mano para montar el caso real.
	if _, err := e.db.Exec(`UPDATE code_memory SET project_id = '' WHERE gist = 'texto del agente '`); err != nil {
		t.Fatal(err)
	}
	antes := generacion(t, e)

	n, err := e.GuardarGistsAutomaticosFrom("musubi", []CodeMemory{autoGist("pkg/a.go", "a.go hace X", "nueva")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("escribió %d filas sobre un archivo con gist de agente: el de agente manda", n)
	}
	filas := filasDelPath(t, e, "pkg/a.go")
	if _, nacio := filas["musubi"]; nacio {
		t.Errorf("nació una fila automática al lado de la del agente: %v", filas)
	}
	if filas[""] != "texto del agente " || filas["Musubi"] != "texto del agente Musubi" {
		t.Errorf("el texto del agente cambió: %v", filas)
	}
	if d := generacion(t, e) - antes; d != 0 {
		t.Errorf("la generación subió %d sin haber escrito nada", d)
	}
}

// Sabotaje: no saltear nunca la fila automática que ya está igual. La llamada repetida vuelve a
// escribir y sube la generación: un push del grafo entero al central en cada tick.
// arnes: archivo="internal/memory/codemem.go"
// arnes: de="if err == nil && fp =="
// arnes: a="if false && fp =="
func TestElGistAutomaticoSinCambiosNoSubeLaGeneracion(t *testing.T) {
	e := newTestEngine(t)
	g := []CodeMemory{autoGist("pkg/a.go", "a.go hace X", "h1")}
	if n, err := e.GuardarGistsAutomaticosFrom("musubi", g, nil); err != nil || n != 1 {
		t.Fatalf("andamio: la primera escritura tenía que dar 1, dio %d (err=%v)", n, err)
	}
	antes := generacion(t, e)
	n, err := e.GuardarGistsAutomaticosFrom("musubi", g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("la segunda escritura idéntica reescribió %d filas", n)
	}
	if d := generacion(t, e) - antes; d != 0 {
		t.Errorf("la generación subió %d sin que cambiara nada: saldría un push del grafo en cada tick", d)
	}
}

// Sabotaje: comparar SÓLO la huella. Con la misma huella y otro texto (lo que pasa cuando sube
// codeintel.VersionResumenDeCabecera) la fila vieja queda con el texto de la regla vieja.
// arnes: archivo="internal/memory/codemem.go"
// arnes: de="fp == cm.Fingerprint && gist == cm.Gist && symbols == cm.Symbols {"
// arnes: a="fp == cm.Fingerprint {"
func TestUnaReglaNuevaReescribeAunqueLaHuellaSeaLaMisma(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.GuardarGistsAutomaticosFrom("musubi", []CodeMemory{autoGist("pkg/a.go", "regla vieja", "h1")}, nil); err != nil {
		t.Fatal(err)
	}
	antes := generacion(t, e)
	n, err := e.GuardarGistsAutomaticosFrom("musubi", []CodeMemory{autoGist("pkg/a.go", "regla nueva", "h1")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("con la misma huella y otro texto escribió %d filas, esperaba 1", n)
	}
	if got := filasDelPath(t, e, "pkg/a.go")["musubi"]; got != PrefijoGistAutomatico+"regla nueva" {
		t.Errorf("la fila quedó con %q: la regla nueva no la alcanzó", got)
	}
	if d := generacion(t, e) - antes; d != 1 {
		t.Errorf("la generación subió %d, esperaba 1: el cambio tiene que viajar al central", d)
	}
}

// Sabotaje: retirar sin mirar la marca. El DELETE se lleva también el gist de agente del mismo
// path y proyecto.
// arnes: archivo="internal/memory/codemem.go"
// arnes: de="`DELETE FROM code_memory WHERE path = ? AND project_id = ? AND substr(gist, 1, length(?)) = ?`,"
// arnes: a="`DELETE FROM code_memory WHERE path = ? AND project_id = ? AND length(?) = length(?)`,"
func TestRetirarBorraSoloLosAutomaticos(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveCodeMemoryFrom("musubi", CodeMemory{Path: "pkg/agente.go", Gist: "del agente", Fingerprint: "h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.GuardarGistsAutomaticosFrom("musubi", []CodeMemory{autoGist("pkg/auto.go", "auto", "h")}, nil); err != nil {
		t.Fatal(err)
	}
	antes := generacion(t, e)
	n, err := e.GuardarGistsAutomaticosFrom("musubi", nil, []string{"pkg/agente.go", "pkg/auto.go", "pkg/no-existe.go"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("retiró %d filas, esperaba 1 (sólo la automática)", n)
	}
	if _, ok := filasDelPath(t, e, "pkg/auto.go")["musubi"]; ok {
		t.Error("el gist automático retirado sigue en la base")
	}
	if got := filasDelPath(t, e, "pkg/agente.go")["musubi"]; got != "del agente" {
		t.Errorf("retirar se llevó el gist de agente: quedó %q", got)
	}
	if d := generacion(t, e) - antes; d != 1 {
		t.Errorf("la generación subió %d, esperaba 1: la baja también tiene que viajar", d)
	}
}

// Sabotaje: contar todo code_memory en CountSavedItems. La siembra de un gist automático se lee
// como «el agente guardó algo» y el recordatorio de captura se pone en cero solo.
// arnes: archivo="internal/memory/operations.go"
// arnes: de="(SELECT COUNT(*) FROM code_memory WHERE substr(gist, 1, length(?)) != ?)`,"
// arnes: a="(SELECT COUNT(*) FROM code_memory WHERE length(?) != length(?) OR 1)`,"
func TestUnGistAutomaticoNoCuentaComoGuardadoDelAgente(t *testing.T) {
	e := newTestEngine(t)
	antes, err := e.CountSavedItems()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.GuardarGistsAutomaticosFrom("", []CodeMemory{autoGist("pkg/a.go", "a", "h"), autoGist("pkg/b.go", "b", "h")}, nil); err != nil {
		t.Fatal(err)
	}
	despues, err := e.CountSavedItems()
	if err != nil {
		t.Fatal(err)
	}
	if despues != antes {
		t.Errorf("CountSavedItems pasó de %d a %d por gists que escribió el índice, no el agente", antes, despues)
	}
	// CONTROL: el gist de un agente sí cuenta. Sin esto, un filtro que excluyera TODO code_memory
	// pasaría la aserción de arriba.
	if err := e.SaveCodeMemory(CodeMemory{Path: "pkg/c.go", Gist: "del agente", Fingerprint: "h"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.CountSavedItems(); n != despues+1 {
		t.Errorf("el gist de un agente no subió el conteo: %d → %d", despues, n)
	}
}

// Sabotaje: sacar el filtro de la marca en CodigoTocadoEnVentana. El gist automático aparece como
// un archivo que alguien re-leyó y re-resumió.
// arnes: archivo="internal/memory/contexto.go"
// arnes: de="AND substr(gist, 1, length(?)) != ?`"
// arnes: a="AND length(?) != length(?) OR 1`"
func TestElCodigoTocadoNoCuentaLosGistsAutomaticos(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.GuardarGistsAutomaticosFrom("", []CodeMemory{autoGist("pkg/auto.go", "a", "h")}, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemory(CodeMemory{Path: "pkg/agente.go", Gist: "del agente", Fingerprint: "h"}); err != nil {
		t.Fatal(err)
	}
	const cuando = "2026-05-05 12:00:00"
	if _, err := e.db.Exec(`UPDATE code_memory SET updated_at = ?`, cuando); err != nil {
		t.Fatal(err)
	}
	desde := time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)
	archivos, err := e.CodigoTocadoEnVentana(context.Background(), fleet.Ventana{Desde: desde, Hasta: desde.Add(2 * time.Hour)}, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) != 1 || archivos[0].Path != "pkg/agente.go" {
		t.Errorf("esperaba sólo el archivo que resumió un agente, obtuve %v", archivos)
	}
}
