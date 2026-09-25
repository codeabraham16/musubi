package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/codeintel"
	"musubi/internal/memory"
)

// gist_automatico_test.go custodia los resúmenes de archivo que se mantienen SOLOS: el índice del
// grafo los saca del comentario de cabecera, los re-escribe cuando el archivo cambia, los retira
// cuando se quedan sin fuente, y nunca le toca el texto a un gist que escribió un agente — al que,
// en cambio, le muestra al lado lo que la cabecera dice hoy.

// proyectoConCabecera arma un repo Go chico con pkg/a.go, cuya cabecera dice `que`.
func proyectoConCabecera(t *testing.T, que string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), conCabecera(que))
	return dir
}

func conCabecera(que string) string {
	return "package pkg\n\n// a.go " + que + ".\n\n// Alpha es la entrada.\nfunc Alpha() { beta() }\n\nfunc beta() {}\n"
}

// vistaDe llama a recall_code y devuelve la respuesta entera, como la leería un cliente.
func vistaDe(t *testing.T, s *McpServer, path string) map[string]interface{} {
	t.Helper()
	res := mustCall(t, s, "musubi_recall_code", map[string]interface{}{"path": path})
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(textOf(t, res)), &out); err != nil {
		t.Fatalf("recall_code no devolvió JSON: %v", err)
	}
	return out
}

// sellarRegla deja la siembra de los gists automáticos al día, para que en la prueba actúe SÓLO
// el refresh por paquete y no el barrido de la primera corrida.
func sellarRegla(t *testing.T, s *McpServer) {
	t.Helper()
	if err := s.engine.SetMeta(memory.MetaGistsAutomaticos, codeintel.VersionResumenDeCabecera); err != nil {
		t.Fatal(err)
	}
}

// Sabotaje: que refreshCodeGraphPkg no escriba los gists del paquete que acaba de re-derivar. Con
// la regla sellada en el andamio, el barrido no actúa, y el primer recall_code da found:false.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if _, gerr := s.engine.GuardarGistsAutomaticosFrom(origin, gists, retirar); gerr != nil {"
// arnes: a="if _, gerr := s.engine.GuardarGistsAutomaticosFrom(origin, gists[:0], retirar[:0]); gerr != nil {"
func TestElIndiceRegeneraElGistAutomaticoCuandoCambiaLaCabecera(t *testing.T) {
	dir := proyectoConCabecera(t, "hace X")
	s := newTestServerWithPath(t, dir)
	sellarRegla(t, s)

	s.reindexCodeGraphOnce(context.Background())
	v := vistaDe(t, s, "pkg/a.go")
	if v["found"] != true {
		t.Fatalf("tras indexar, pkg/a.go no tiene gist automático: %v", v)
	}
	if g, _ := v["gist"].(string); !strings.HasPrefix(g, memory.PrefijoGistAutomatico) || !strings.Contains(g, "hace X") {
		t.Errorf("gist = %q, esperaba la cabecera con la marca de automático", g)
	}
	if v["origen"] != origenCabecera || v["freshness"] != frescuraFresca {
		t.Errorf("origen=%v freshness=%v, esperaba %q y %q", v["origen"], v["freshness"], origenCabecera, frescuraFresca)
	}

	writeFile(t, filepath.Join(dir, "pkg", "a.go"), conCabecera("hace Y"))
	s.reindexCodeGraphOnce(context.Background())
	v = vistaDe(t, s, "pkg/a.go")
	if g, _ := v["gist"].(string); !strings.Contains(g, "hace Y") {
		t.Errorf("la cabecera cambió y el gist sigue diciendo %q", g)
	}
	if v["freshness"] != frescuraFresca {
		t.Errorf("el gist regenerado salió %v: tenía que quedar con la huella del contenido nuevo", v["freshness"])
	}
}

// Sabotaje: sacar la siembra pendiente de sinCambios(). El scheduler ve el árbol limpio —el grafo
// ya estaba indexado— y el archivo que no cambió no recibe su gist nunca.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="&& !p.derivadorCambio && !p.gistsAutoPendientes"
// arnes: a="&& !p.derivadorCambio"
func TestLaPrimeraCorridaSiembraArchivosQueNoCambiaron(t *testing.T) {
	dir := proyectoConCabecera(t, "hace X")
	s := newTestServerWithPath(t, dir)
	s.reindexCodeGraphOnce(context.Background())
	// Se monta una base indexada por un binario VIEJO: grafo al día, sin gists automáticos y sin
	// el sello de la regla.
	if _, err := s.engine.GuardarGistsAutomaticosFrom("", nil, []string{"pkg/a.go"}); err != nil {
		t.Fatal(err)
	}
	if err := s.engine.SetMeta(memory.MetaGistsAutomaticos, "0-binario-viejo"); err != nil {
		t.Fatal(err)
	}
	if v := vistaDe(t, s, "pkg/a.go"); v["found"] != false {
		t.Fatalf("andamio: el gist tenía que haberse retirado, quedó %v", v)
	}

	s.reindexCodeGraphOnce(context.Background())
	if v := vistaDe(t, s, "pkg/a.go"); v["found"] != true {
		t.Fatalf("el archivo no cambió y la primera corrida no le sembró el gist: %v", v)
	}
	if plan, err := s.planearIncremental(context.Background()); err != nil || !plan.sinCambios() {
		t.Errorf("tras la siembra el plan tenía que quedar sin cambios (si no, se barre en cada tick): %+v err=%v", plan, err)
	}
}

// Sabotaje: no mostrar la cabecera al lado del gist de agente. recall_code devuelve sólo el texto
// rancio, que es exactamente lo que la decisión del dueño (opción c) vino a completar.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\t\t\t\tv[\"cabecera\"] = c\n"
// arnes: a="\t\t\t\t_ = c\n"
func TestUnGistDeAgenteRancioSeConservaYMuestraLaCabeceraAlDia(t *testing.T) {
	dir := proyectoConCabecera(t, "hace X")
	s := newTestServerWithPath(t, dir)
	s.reindexCodeGraphOnce(context.Background())
	mustCall(t, s, "musubi_save_code", map[string]interface{}{"path": "pkg/a.go", "gist": "texto del agente"})

	writeFile(t, filepath.Join(dir, "pkg", "a.go"), conCabecera("hace Y"))
	s.reindexCodeGraphOnce(context.Background())

	v := vistaDe(t, s, "pkg/a.go")
	if v["gist"] != "texto del agente" {
		t.Fatalf("el índice tocó el texto del agente: gist=%v", v["gist"])
	}
	if v["freshness"] != frescuraRancia || v["origen"] != origenAgente {
		t.Errorf("freshness=%v origen=%v, esperaba %q y %q: el índice no le refresca la huella",
			v["freshness"], v["origen"], frescuraRancia, origenAgente)
	}
	if c, _ := v["cabecera"].(string); !strings.Contains(c, "hace Y") {
		t.Errorf("recall_code no trajo al lado la cabecera al día: cabecera=%q", c)
	}

	res := mustCall(t, s, "musubi_code_context", map[string]interface{}{"symbol": "pkg/a.go#func:Alpha"})
	var cc struct {
		FileGist map[string]interface{} `json:"file_gist"`
	}
	if err := json.Unmarshal([]byte(textOf(t, res)), &cc); err != nil {
		t.Fatal(err)
	}
	if cc.FileGist["gist"] != "texto del agente" {
		t.Errorf("code_context no trajo el gist del archivo: %v", cc.FileGist)
	}
	if c, _ := cc.FileGist["cabecera"].(string); !strings.Contains(c, "hace Y") {
		t.Errorf("code_context no trajo la cabecera al día al lado: %v", cc.FileGist)
	}
}

// Sabotaje: que gistsAutomaticosDe no anote para retirar los archivos que perdieron la cabecera.
// El gist automático de a.go queda con el texto viejo para siempre.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="\t\t\tretirar = append(retirar, key)\n"
// arnes: a=""
func TestBorrarLaCabeceraRetiraElGistAutomatico(t *testing.T) {
	dir := proyectoConCabecera(t, "hace X")
	s := newTestServerWithPath(t, dir)
	s.reindexCodeGraphOnce(context.Background())
	if v := vistaDe(t, s, "pkg/a.go"); v["found"] != true {
		t.Fatalf("andamio: pkg/a.go tenía que tener gist automático: %v", v)
	}

	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")
	s.reindexCodeGraphOnce(context.Background())
	if v := vistaDe(t, s, "pkg/a.go"); v["found"] != false {
		t.Errorf("la cabecera se borró y el gist automático sigue: %v", v)
	}
}

// Sabotaje: no retirar los gists de los archivos fantasma. b.go se borra del disco, la poda saca
// sus nodos del grafo, y su gist automático queda huérfano viajando al central.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="retirar := append([]string{}, plan.ghostPaths...)"
// arnes: a="retirar := append([]string{}, plan.ghostPaths[:0]...)"
func TestBorrarElArchivoRetiraSuGistAutomatico(t *testing.T) {
	dir := proyectoConCabecera(t, "hace X")
	writeFile(t, filepath.Join(dir, "pkg", "b.go"), "// Package pkg: b.go hace Z.\npackage pkg\n\nfunc Beta() {}\n")
	s := newTestServerWithPath(t, dir)
	s.reindexCodeGraphOnce(context.Background())
	if v := vistaDe(t, s, "pkg/b.go"); v["found"] != true {
		t.Fatalf("andamio: pkg/b.go tenía que tener gist automático: %v", v)
	}

	if err := os.Remove(filepath.Join(dir, "pkg", "b.go")); err != nil {
		t.Fatal(err)
	}
	s.reindexCodeGraphOnce(context.Background())
	if v := vistaDe(t, s, "pkg/b.go"); v["found"] != false {
		t.Errorf("el archivo se borró y su gist automático sigue: %v", v)
	}
	if v := vistaDe(t, s, "pkg/a.go"); v["found"] != true {
		t.Errorf("control: el gist del archivo que sigue vivo no tenía que irse: %v", v)
	}
}

// Sabotaje: que save_code guarde el gist tal cual llega. Un agente que copia la marca de
// automático deja su texto expuesto a que el índice lo pise en el próximo tick.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\targs.Gist = memory.SinPrefijoAutomatico(args.Gist)\n"
// arnes: a=""
func TestSaveCodeLeSacaLaMarcaAlGistDelAgente(t *testing.T) {
	dir := proyectoConCabecera(t, "hace X")
	s := newTestServerWithPath(t, dir)
	sellarRegla(t, s)
	mustCall(t, s, "musubi_save_code", map[string]interface{}{
		"path": "pkg/a.go", "gist": memory.PrefijoGistAutomatico + "texto del agente",
	})
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), conCabecera("hace Y"))
	s.reindexCodeGraphOnce(context.Background())

	v := vistaDe(t, s, "pkg/a.go")
	if v["gist"] != "texto del agente" || v["origen"] != origenAgente {
		t.Errorf("gist=%v origen=%v: el texto del agente tenía que quedar sin la marca y sin pisar", v["gist"], v["origen"])
	}
}
