package memory

import (
	"context"
	"testing"
)

// ReplaceProjectCodeMemoryFrom borra antes de insertar. Ese DELETE es el punto peligroso: si no
// estuviera scopeado por project_id, federar un proyecto le borraría los gists a los demás. Y no
// es teórico — el token de mando del cerebro central es write=any, así que puede declarar destino.
func TestReemplazarGistsNoTocaLosDeOtroProyecto(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveCodeMemoryFrom("altura", CodeMemory{Path: "src/a.ts", Gist: "gist de altura", Fingerprint: "a1"}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemoryFrom("musubi", CodeMemory{Path: "cmd/x.go", Gist: "gist viejo de musubi", Fingerprint: "m1"}); err != nil {
		t.Fatal(err)
	}

	// Federar musubi: reemplaza LO SUYO y nada más.
	nuevos := []CodeMemory{{Path: "cmd/y.go", Gist: "gist nuevo de musubi", Fingerprint: "m2"}}
	if err := e.ReplaceProjectCodeMemoryFrom("musubi", nuevos); err != nil {
		t.Fatal(err)
	}

	deAltura := gistsDeProyecto(t, e, "altura")
	if len(deAltura) != 1 || deAltura[0].Gist != "gist de altura" {
		t.Errorf("federar musubi tocó los gists de altura: %+v", deAltura)
	}
	deMusubi := gistsDeProyecto(t, e, "musubi")
	if len(deMusubi) != 1 || deMusubi[0].Path != "cmd/y.go" {
		t.Errorf("musubi debería tener sólo el gist nuevo, tiene %+v", deMusubi)
	}
}

// Una fila mal formada del emisor no puede abortar la federación entera: se saltea y el resto entra.
func TestGistSinPathOSinContenidoSeSaltea(t *testing.T) {
	e := newTestEngine(t)
	entrada := []CodeMemory{
		{Path: "ok.go", Gist: "este sirve"},
		{Path: "", Gist: "sin path"},
		{Path: "sin-gist.go", Gist: ""},
	}
	if err := e.ReplaceProjectCodeMemoryFrom("musubi", entrada); err != nil {
		t.Fatalf("una fila mal formada no debe abortar el reemplazo: %v", err)
	}
	if g := gistsDeProyecto(t, e, "musubi"); len(g) != 1 || g[0].Path != "ok.go" {
		t.Errorf("esperaba sólo el gist bien formado, hay %+v", g)
	}
}

func gistsDeProyecto(t *testing.T, e *DbEngine, proyecto string) []CodeMemory {
	t.Helper()
	ctx := WithProjectScope(context.Background(), ProjectScope{ProjectID: proyecto})
	g, err := e.AllCodeMemoryCtx(ctx)
	if err != nil {
		t.Fatalf("AllCodeMemoryCtx(%s): %v", proyecto, err)
	}
	return g
}

// EL DESEMPATE. La tabla admite dos gists del mismo archivo —la PK es (path, project_id)— y en una
// base real conviven: los anteriores a la atribución multi-tenant quedaron con project_id='' y el
// mismo archivo volvió a gistearse después con el suyo. Medido en altura-erp el 2026-08-12: 25
// filas, 23 paths, 2 duplicados (uno de junio sin atribuir, otro de julio atribuido).
//
// Al federar los dos, ganaba el ÚLTIMO que insertara el receptor y el orden entre filas de igual
// path no estaba definido. En la prueba real ganó el correcto POR CASUALIDAD. Este test fija la
// regla: gana el del proyecto, y va uno solo por path.
func TestAlFederarGanaElGistDelProyectoNoElSinAtribuir(t *testing.T) {
	e := newTestEngine(t)
	const p = "src/components/Layout.jsx"

	if err := e.SaveCodeMemoryFrom("", CodeMemory{Path: p, Gist: "gist VIEJO sin atribuir", Fingerprint: "junio"}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemoryFrom("altura", CodeMemory{Path: p, Gist: "gist NUEVO del proyecto", Fingerprint: "julio"}); err != nil {
		t.Fatal(err)
	}

	g := gistsDeProyecto(t, e, "altura")
	if len(g) != 1 {
		t.Fatalf("esperaba UN solo gist por path, hay %d: %+v", len(g), g)
	}
	if g[0].Fingerprint != "julio" {
		t.Errorf("ganó el gist %q (%s); debe ganar el del proyecto, no el sin atribuir",
			g[0].Gist, g[0].Fingerprint)
	}
}

// Y el desempate no puede cambiar según el orden en que se guardaron: da igual cuál se escribió
// primero. Si dependiera del rowid, un VACUUM lo invertiría.
func TestElDesempateNoDependeDelOrdenDeEscritura(t *testing.T) {
	e := newTestEngine(t)
	const p = "src/pages/ReportesFichaje.jsx"

	// Ahora al revés: primero el atribuido, después el sin atribuir.
	if err := e.SaveCodeMemoryFrom("altura", CodeMemory{Path: p, Gist: "del proyecto", Fingerprint: "julio"}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemoryFrom("", CodeMemory{Path: p, Gist: "sin atribuir", Fingerprint: "junio"}); err != nil {
		t.Fatal(err)
	}

	g := gistsDeProyecto(t, e, "altura")
	if len(g) != 1 || g[0].Fingerprint != "julio" {
		t.Errorf("el desempate cambió con el orden de escritura: %+v", g)
	}
}
