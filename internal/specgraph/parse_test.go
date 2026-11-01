package specgraph

// Pruebas de parse.go: el split del frontmatter, la normalización a escalares/
// listas, la definición de "esto es una spec" (IsSpec) y la serialización del
// camino create. Cada tabla cubre un invariante que el port promete sobre
// parse.ts (R1, R2, R4) más la creación (R5 es de frontmatter_edit).

import (
	"reflect"
	"strings"
	"testing"
)

// conBOM antepone el BOM UTF-8 al texto, para los casos que prueban que se
// descarta. Se arma con la constante del paquete para no reescribir los bytes.
func conBOM(s string) string { return bom + s }

// TestParseFile fija qué sale de partir un archivo en frontmatter + cuerpo:
// escalares, listas en bloque y en flow, los tres caminos que dan Frontmatter
// nil (sin fence, YAML roto, raíz no-objeto) y la preservación del cuerpo con
// sus \r bajo CRLF.
//
// Sabotaje que la hace fallar: que splitFrontmatter deje de descartar el BOM, y
// entonces un archivo con BOM deja de reconocerse como spec (lines[0] ya no
// iguala al fence).
// arnes: archivo="internal/specgraph/parse.go"
// arnes: de="normalized := strings.TrimPrefix(content, bom)"
// arnes: a="normalized := content"
func TestParseFile(t *testing.T) {
	// cuerpo multilínea CRLF: comprueba que el \r del medio sobrevive byte a byte.
	crlf := "---\r\nid: t1\r\ntype: task-spec\r\n---\r\nlínea a\r\nlínea b"

	casos := []struct {
		nombre    string
		contenido string
		wantNil   bool           // Frontmatter debe ser nil
		wantFM    map[string]any // esperado si no es nil (comparado con DeepEqual)
		wantBody  string
	}{
		{
			nombre:    "escalares",
			contenido: "---\nid: t1\ntype: task-spec\ntitle: Tarea\n---\ncuerpo",
			wantFM:    map[string]any{"id": "t1", "type": "task-spec", "title": "Tarea"},
			wantBody:  "cuerpo",
		},
		{
			nombre:    "lista en bloque",
			contenido: "---\nid: t1\ntype: task-spec\ntags:\n  - a\n  - b\n---\ncuerpo",
			wantFM:    map[string]any{"id": "t1", "type": "task-spec", "tags": []string{"a", "b"}},
			wantBody:  "cuerpo",
		},
		{
			nombre:    "lista en flow",
			contenido: "---\nid: t1\ntype: task-spec\ntags: [a, b]\n---\ncuerpo",
			wantFM:    map[string]any{"id": "t1", "type": "task-spec", "tags": []string{"a", "b"}},
			wantBody:  "cuerpo",
		},
		{
			// null por clave: se saltea (no queda la clave con valor nil).
			nombre:    "valor null salteado",
			contenido: "---\nid: t1\ntype: task-spec\nstatus:\n---\ncuerpo",
			wantFM:    map[string]any{"id": "t1", "type": "task-spec"},
			wantBody:  "cuerpo",
		},
		{
			// objeto anidado: scalarString lo rechaza, así que la clave se descarta.
			nombre:    "objeto anidado descartado",
			contenido: "---\nid: t1\ntype: task-spec\nmeta:\n  a: 1\n---\ncuerpo",
			wantFM:    map[string]any{"id": "t1", "type": "task-spec"},
			wantBody:  "cuerpo",
		},
		{
			nombre:    "BOM se descarta",
			contenido: conBOM("---\nid: t1\ntype: task-spec\n---\ncuerpo"),
			wantFM:    map[string]any{"id": "t1", "type": "task-spec"},
			wantBody:  "cuerpo",
		},
		{
			nombre:    "CRLF conserva el cuerpo",
			contenido: crlf,
			wantFM:    map[string]any{"id": "t1", "type": "task-spec"},
			wantBody:  "línea a\r\nlínea b",
		},
		{
			// frontmatter vacío: mapa NO-nil (hubo fence) pero vacío ⇒ no es spec.
			nombre:    "frontmatter vacío",
			contenido: "---\n---\ncuerpo",
			wantFM:    map[string]any{},
			wantBody:  "cuerpo",
		},
		{
			// sin fence: Frontmatter nil y el cuerpo es el archivo entero.
			nombre:    "sin fence",
			contenido: "no hay frontmatter acá\nsegunda línea",
			wantNil:   true,
			wantBody:  "no hay frontmatter acá\nsegunda línea",
		},
		{
			// YAML malformado: nil y el cuerpo vuelve a ser el archivo ENTERO
			// (no "lo que sigue al fence"), igual que thinkrail.
			nombre:    "YAML malformado",
			contenido: "---\nid: [sin cerrar\n---\ncuerpo",
			wantNil:   true,
			wantBody:  "---\nid: [sin cerrar\n---\ncuerpo",
		},
		{
			// raíz que no es objeto (una lista): Frontmatter nil.
			nombre:    "raíz no-objeto",
			contenido: "---\n- a\n- b\n---\ncuerpo",
			wantNil:   true,
			wantBody:  "cuerpo",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := ParseFile(c.contenido)
			if c.wantNil {
				if got.Frontmatter != nil {
					t.Errorf("Frontmatter: quería nil, dio %#v", got.Frontmatter)
				}
			} else {
				if !reflect.DeepEqual(map[string]any(got.Frontmatter), c.wantFM) {
					t.Errorf("Frontmatter:\n  dio    %#v\n  quería %#v", got.Frontmatter, c.wantFM)
				}
			}
			if got.Body != c.wantBody {
				t.Errorf("Body:\n  dio    %q\n  quería %q", got.Body, c.wantBody)
			}
		})
	}
}

// TestSplitFrontmatter fija los bordes del partidor: el fence tiene que estar en
// la primera línea y tiene que haber un cierre, o no hay frontmatter y el cuerpo
// es el contenido original intacto.
func TestSplitFrontmatter(t *testing.T) {
	casos := []struct {
		nombre    string
		contenido string
		wantHas   bool
		wantFM    string
		wantBody  string
	}{
		{"fence y cierre", "---\nid: x\n---\ncuerpo", true, "id: x", "cuerpo"},
		{"sin cierre", "---\nid: x\nsin cierre", false, "", "---\nid: x\nsin cierre"},
		{"fence no en la primera línea", "prólogo\n---\nid: x\n---\n", false, "", "prólogo\n---\nid: x\n---\n"},
		{"CRLF: el fmText pierde los \\r", "---\r\nid: x\r\n---\r\ncuerpo", true, "id: x", "cuerpo"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fm, has, body := splitFrontmatter(c.contenido)
			if has != c.wantHas {
				t.Fatalf("hasFM: dio %v, quería %v", has, c.wantHas)
			}
			if has && fm != c.wantFM {
				t.Errorf("fmText: dio %q, quería %q", fm, c.wantFM)
			}
			if body != c.wantBody {
				t.Errorf("body: dio %q, quería %q", body, c.wantBody)
			}
		})
	}
}

// TestIsSpec fija R1: "esto es una spec" es EXACTAMENTE id+type como escalares no
// vacíos, ni más (title no decide) ni menos.
//
// Sabotaje que la hace fallar: que IsSpec exija los RequiredFields (que incluyen
// title) en vez de los IdentityFields, y entonces una spec sin title deja de
// reconocerse como spec.
// arnes: archivo="internal/specgraph/parse.go"
// arnes: de="for _, field := range IdentityFields {"
// arnes: a="for _, field := range RequiredFields {"
func TestIsSpec(t *testing.T) {
	casos := []struct {
		nombre string
		fm     Frontmatter
		want   bool
	}{
		{"id y type", Frontmatter{"id": "x", "type": "task-spec"}, true},
		{"id, type y title", Frontmatter{"id": "x", "type": "task-spec", "title": "T"}, true},
		{"sin title igual es spec", Frontmatter{"id": "x", "type": "goal-and-requirements"}, true},
		{"falta id", Frontmatter{"type": "task-spec"}, false},
		{"falta type", Frontmatter{"id": "x"}, false},
		{"id vacío", Frontmatter{"id": "", "type": "task-spec"}, false},
		{"id es lista", Frontmatter{"id": []string{"x"}, "type": "task-spec"}, false},
		{"nil", nil, false},
		{"vacío", Frontmatter{}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := IsSpec(c.fm); got != c.want {
				t.Errorf("IsSpec(%#v) = %v, quería %v", c.fm, got, c.want)
			}
		})
	}
}

// TestScalarYList fija los accesos tipados: Scalar sólo devuelve strings no
// vacíos; List normaliza un escalar a lista de uno y una ausencia a nil.
func TestScalarYList(t *testing.T) {
	fm := Frontmatter{
		"id":         "x",
		"type":       "",
		"tags":       []string{"a", "b"},
		"depends-on": "solo",
		"vacía":      []string{},
	}

	t.Run("Scalar", func(t *testing.T) {
		if v, ok := Scalar(fm, "id"); !ok || v != "x" {
			t.Errorf("Scalar(id) = %q,%v; quería x,true", v, ok)
		}
		if _, ok := Scalar(fm, "type"); ok {
			t.Error("Scalar(type) de un string vacío debe dar ok=false")
		}
		if _, ok := Scalar(fm, "tags"); ok {
			t.Error("Scalar(tags) de una lista debe dar ok=false")
		}
		if _, ok := Scalar(fm, "ausente"); ok {
			t.Error("Scalar de una clave ausente debe dar ok=false")
		}
	})

	t.Run("List", func(t *testing.T) {
		if got := List(fm, "tags"); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Errorf("List(tags) = %#v; quería [a b]", got)
		}
		if got := List(fm, "depends-on"); !reflect.DeepEqual(got, []string{"solo"}) {
			t.Errorf("List(depends-on) de un escalar = %#v; quería [solo]", got)
		}
		if got := List(fm, "ausente"); got != nil {
			t.Errorf("List de una clave ausente = %#v; quería nil", got)
		}
	})
}

// TestSerializeFrontmatterRoundTrip fija el camino create: serializar una spec y
// volver a parsearla devuelve los mismos valores, los campos vacíos se descartan,
// y un item con coma sobrevive como UN item (no se parte).
func TestSerializeFrontmatterRoundTrip(t *testing.T) {
	t.Run("round-trip y descarte de vacíos", func(t *testing.T) {
		in := Frontmatter{
			"id":         "mod-1",
			"type":       "module-design",
			"status":     "", // vacío ⇒ debe descartarse
			"title":      "Un módulo",
			"depends-on": []string{"goal-1"},
			"tags":       []string{}, // lista vacía ⇒ debe descartarse
		}
		out := SerializeFrontmatter(in)
		re := ParseFile(out)
		want := map[string]any{
			"id":         "mod-1",
			"type":       "module-design",
			"title":      "Un módulo",
			"depends-on": []string{"goal-1"},
		}
		if !reflect.DeepEqual(map[string]any(re.Frontmatter), want) {
			t.Errorf("round-trip:\n  dio    %#v\n  quería %#v\n  texto: %q", re.Frontmatter, want, out)
		}
		if _, ok := re.Frontmatter["status"]; ok {
			t.Error("status vacío no debía serializarse")
		}
		if _, ok := re.Frontmatter["tags"]; ok {
			t.Error("tags vacío no debía serializarse")
		}
	})

	t.Run("item con coma no se parte", func(t *testing.T) {
		in := Frontmatter{
			"id":         "x",
			"type":       "task-spec",
			"depends-on": []string{"a,b", "c"},
		}
		re := ParseFile(SerializeFrontmatter(in))
		if got := List(re.Frontmatter, "depends-on"); !reflect.DeepEqual(got, []string{"a,b", "c"}) {
			t.Errorf("depends-on tras round-trip = %#v; quería [\"a,b\" \"c\"]", got)
		}
	})

	t.Run("frontmatter vacío", func(t *testing.T) {
		if got := SerializeFrontmatter(Frontmatter{}); got != "---\n---\n" {
			t.Errorf("frontmatter vacío = %q; quería \"---\\n---\\n\"", got)
		}
	})

	t.Run("claves desconocidas van después y ordenadas", func(t *testing.T) {
		// zeta y alfa no están en FieldOrder: deben salir al final, alfabéticas.
		out := SerializeFrontmatter(Frontmatter{
			"id": "x", "type": "task-spec", "zeta": "2", "alfa": "1",
		})
		iAlfa := strings.Index(out, "alfa")
		iZeta := strings.Index(out, "zeta")
		iType := strings.Index(out, "type")
		if !(iType < iAlfa && iAlfa < iZeta) {
			t.Errorf("orden esperado type < alfa < zeta; dio índices type=%d alfa=%d zeta=%d\n%s", iType, iAlfa, iZeta, out)
		}
	})
}
