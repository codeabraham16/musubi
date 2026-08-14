package memory

import (
	"context"
	"testing"
)

// Tests de ListGraphFuncsInDirsCtx (Track 20 · F8-A): la consulta que permite resolver una llamada
// CROSS-PAQUETE en el refresco incremental. De un import path se conoce el DIRECTORIO del paquete,
// nunca el ARCHIVO donde vive el símbolo, así que ni el lookup exacto por node_key ni el listado por
// archivo alcanzaban.

// sembrarPorDirs siembra funcs en tres directorios más la raíz, incluyendo un dir con guion bajo:
// `internal/a_b` existe para probar el escape de LIKE, donde `_` es COMODÍN DE UN CARÁCTER.
func sembrarPorDirs(t *testing.T, e *DbEngine) {
	t.Helper()
	nodes := []GraphNode{
		{Key: "internal/util/util.go#func:Ayuda", Kind: "func", Name: "Ayuda", Path: "internal/util/util.go"},
		{Key: "internal/util/otro.go#func:Segunda", Kind: "func", Name: "Segunda", Path: "internal/util/otro.go"},
		{Key: "internal/util/util.go#type:Config", Kind: "type", Name: "Config", Path: "internal/util/util.go"},
		{Key: "internal/util/sub/hondo.go#func:Hondo", Kind: "func", Name: "Hondo", Path: "internal/util/sub/hondo.go"},
		{Key: "internal/a_b/x.go#func:ConGuion", Kind: "func", Name: "ConGuion", Path: "internal/a_b/x.go"},
		{Key: "internal/axb/x.go#func:SinGuion", Kind: "func", Name: "SinGuion", Path: "internal/axb/x.go"},
		{Key: "raiz.go#func:EnLaRaiz", Kind: "func", Name: "EnLaRaiz", Path: "raiz.go"},
	}
	files := []string{
		"internal/util/util.go", "internal/util/otro.go", "internal/util/sub/hondo.go",
		"internal/a_b/x.go", "internal/axb/x.go", "raiz.go",
	}
	if err := e.UpsertPackageGraphFrom("", files, nodes, nil); err != nil {
		t.Fatal(err)
	}
}

func motorSembrado(t *testing.T) *DbEngine {
	t.Helper()
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	sembrarPorDirs(t, e)
	return e
}

func nombresDe(ns []GraphNode) map[string]bool {
	out := map[string]bool{}
	for _, n := range ns {
		out[n.Name] = true
	}
	return out
}

// Lo básico y lo que más importa: trae las funcs del dir, de TODOS sus archivos, y NO baja a los
// subdirectorios. En Go un paquete es un directorio, no un árbol: si bajara, mezclaría los símbolos
// de un paquete con los de otro y la resolución apuntaría al archivo equivocado.
func TestListGraphFuncsInDirs_NoEsRecursivo(t *testing.T) {
	got, err := motorSembrado(t).ListGraphFuncsInDirsCtx(context.Background(), []string{"internal/util"})
	if err != nil {
		t.Fatal(err)
	}
	n := nombresDe(got)
	if !n["Ayuda"] || !n["Segunda"] {
		t.Errorf("faltan funcs del dir (esperaba Ayuda y Segunda de dos archivos distintos): %v", n)
	}
	if n["Hondo"] {
		t.Error("trajo una func de un SUBdirectorio: mezclaría los símbolos de dos paquetes")
	}
	if n["Config"] {
		t.Error("trajo un type; sólo las funcs top-level son invocables como paquete.Func()")
	}
	if len(got) != 2 {
		t.Errorf("esperaba exactamente 2 nodos, obtuve %d: %+v", len(got), got)
	}
}

// ⚠️ EL CASO SUTIL, y la razón de que la consulta use ESCAPE. En LIKE, `_` matchea CUALQUIER
// carácter, así que sin escapar el dir `internal/a_b` también traería lo de `internal/axb` —
// símbolos de OTRO paquete resolviendo silenciosamente hacia el equivocado.
func TestListGraphFuncsInDirs_GuionBajoNoEsComodin(t *testing.T) {
	got, err := motorSembrado(t).ListGraphFuncsInDirsCtx(context.Background(), []string{"internal/a_b"})
	if err != nil {
		t.Fatal(err)
	}
	n := nombresDe(got)
	if !n["ConGuion"] {
		t.Error("no trajo la func del dir pedido")
	}
	if n["SinGuion"] {
		t.Error("el `_` actuó como comodín y trajo internal/axb: falta el ESCAPE en el LIKE")
	}
}

// La raíz del módulo es "." y no cadena vacía — así la nombra el resto del indexador. Sus archivos
// no tienen barra, que es justo lo que la consulta usa para distinguirlos.
func TestListGraphFuncsInDirs_Raiz(t *testing.T) {
	e := motorSembrado(t)
	got, err := e.ListGraphFuncsInDirsCtx(context.Background(), []string{"."})
	if err != nil {
		t.Fatal(err)
	}
	n := nombresDe(got)
	if !n["EnLaRaiz"] {
		t.Errorf("la raíz no devolvió su func: %+v", got)
	}
	if n["Ayuda"] {
		t.Error("la raíz se llevó funcs de subdirectorios")
	}
}

// Varios dirs en una sola consulta: es como la usa el refresco incremental, que pide de una todos
// los paquetes in-module que aparecen en sus pendientes.
func TestListGraphFuncsInDirs_VariosDirsYVacio(t *testing.T) {
	e := motorSembrado(t)
	ctx := context.Background()

	got, err := e.ListGraphFuncsInDirsCtx(ctx, []string{"internal/util", "internal/a_b", "."})
	if err != nil {
		t.Fatal(err)
	}
	n := nombresDe(got)
	for _, q := range []string{"Ayuda", "Segunda", "ConGuion", "EnLaRaiz"} {
		if !n[q] {
			t.Errorf("falta %s al pedir varios dirs juntos: %v", q, n)
		}
	}
	if n["Hondo"] || n["SinGuion"] {
		t.Errorf("se coló algo que no se pidió: %v", n)
	}

	// Sin dirs no hay consulta que hacer: nil, no un barrido de todo el proyecto.
	vacio, err := e.ListGraphFuncsInDirsCtx(ctx, nil)
	if err != nil || vacio != nil {
		t.Errorf("sin dirs esperaba (nil, nil), obtuve (%+v, %v)", vacio, err)
	}

	// Un dir inexistente no es un error: simplemente no hay candidatos y el pendiente se omite.
	nada, err := e.ListGraphFuncsInDirsCtx(ctx, []string{"internal/no/existe"})
	if err != nil {
		t.Fatalf("un dir sin nodos no debe fallar: %v", err)
	}
	if len(nada) != 0 {
		t.Errorf("esperaba cero nodos, obtuve %+v", nada)
	}
}
