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
