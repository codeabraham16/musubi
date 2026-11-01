package specgraph

// Pruebas de frontmatter_edit.go: la edición quirúrgica tiene que ser LOSSLESS
// (R5/R6) —comentarios, orden de claves, BOM, CRLF y cuerpo byte a byte— y
// tiene que RECHAZAR las cuatro ediciones que romperían una spec: renombrar id,
// editar una lista con set, borrar id/type, o dejar el archivo sin id+type.

import (
	"errors"
	"strings"
	"testing"
)

// TestUpdateFrontmatterLossless fija R5/R6: una edición que toca una clave deja
// intacto todo lo demás. El comentario de cabecera, el comentario por-item de
// una lista en BLOQUE que no se tocó, el orden de las claves y el cuerpo en prosa
// sobreviven; sólo cambia lo que la edición pidió.
func TestUpdateFrontmatterLossless(t *testing.T) {
	in := strings.Join([]string{
		"---",
		"# cabecera que explica la spec",
		"id: t1",
		"type: task-spec",
		"status: draft",
		"tags:",
		"  - a # el primer tag",
		"  - b",
		"---",
		"cuerpo en prosa",
		"con una segunda línea",
	}, "\n")
	_, _, bodyOriginal := splitFrontmatter(in)

	out, err := UpdateFrontmatterText(in, FrontmatterEdit{Set: map[string]string{FieldStatus: "active"}})
	if err != nil {
		t.Fatalf("edición válida devolvió error: %v", err)
	}

	// El cambio pedido se aplicó.
	if !strings.Contains(out, "status: active") {
		t.Errorf("no aplicó status: active:\n%s", out)
	}
	// R5: el comentario de cabecera sobrevive.
	if !strings.Contains(out, "# cabecera que explica la spec") {
		t.Errorf("perdió el comentario de cabecera:\n%s", out)
	}
	// R5: la lista en bloque que NO se tocó conserva su estilo (el guion) y su
	// comentario por-item — ésa es la divergencia deliberada con thinkrail, que
	// forzaría flow sobre toda lista y se comería el comentario.
	if !strings.Contains(out, "- a") {
		t.Errorf("reflowó a flow una lista que no se tocó (se perdió el estilo bloque):\n%s", out)
	}
	if !strings.Contains(out, "# el primer tag") {
		t.Errorf("perdió el comentario por-item de la lista no tocada:\n%s", out)
	}
	// Orden de claves conservado.
	iID, iType, iStatus := strings.Index(out, "id: t1"), strings.Index(out, "type: task-spec"), strings.Index(out, "status: active")
	if !(iID < iType && iType < iStatus) {
		t.Errorf("se alteró el orden de las claves: id=%d type=%d status=%d\n%s", iID, iType, iStatus, out)
	}
	// R6: el cuerpo en prosa es byte a byte idéntico.
	if _, _, bodyOut := splitFrontmatter(out); bodyOut != bodyOriginal {
		t.Errorf("el cuerpo cambió:\n  dio    %q\n  quería %q", bodyOut, bodyOriginal)
	}
}

// TestUpdateFrontmatterPreservaBOMyCRLF fija que la edición conserva el BOM y el
// terminador de línea CRLF del archivo original, y que el cuerpo round-trippea.
func TestUpdateFrontmatterPreservaBOMyCRLF(t *testing.T) {
	in := bom + strings.Join([]string{"---", "id: t1", "type: task-spec", "status: draft", "---", "cuerpo"}, "\r\n")

	out, err := UpdateFrontmatterText(in, FrontmatterEdit{Set: map[string]string{FieldStatus: "active"}})
	if err != nil {
		t.Fatalf("edición válida devolvió error: %v", err)
	}
	if !strings.HasPrefix(out, bom) {
		t.Error("se perdió el BOM del original")
	}
	if lineEnding(out) != crlf {
		t.Error("el terminador de línea dejó de ser CRLF")
	}
	// Cada \n del archivo tiene que venir en un par \r\n: si quedara un \n suelto,
	// el archivo mezclaría terminadores. Se neutralizan los pares y no debe quedar \n.
	if sinPares := strings.ReplaceAll(strings.TrimPrefix(out, bom), crlf, ""); strings.Contains(sinPares, "\n") {
		t.Errorf("quedó un \\n sin su \\r (terminadores mezclados):\n%q", out)
	}
	if !strings.Contains(out, "status: active") {
		t.Errorf("no aplicó el cambio bajo CRLF:\n%q", out)
	}
	if _, _, body := splitFrontmatter(out); body != "cuerpo" {
		t.Errorf("el cuerpo no round-trippeó bajo CRLF: %q", body)
	}
}

// TestUpdateFrontmatterGuards fija las cuatro guardas que protegen la integridad
// de una spec. Las dos saboteadas abajo son las de identidad: sin ellas, una
// edición podría renombrar el id (rompiendo todas las aristas que lo apuntan) o
// dejar el archivo sin un type válido (dejándolo de ser spec en silencio).
//
// Sabotaje de la guarda anti-rename: que set deje de bloquear la clave id.
// arnes: archivo="internal/specgraph/frontmatter_edit.go"
// arnes: de="if key == FieldID {"
// arnes: a="if key == \"\" {"
//
// Sabotaje de la guarda "no dejar inválido": que invierta la condición y pase a
// aceptar justo las ediciones que dejan al archivo sin id+type.
// arnes: archivo="internal/specgraph/frontmatter_edit.go"
// arnes: de="if _, ok := fmDocScalar(mapping, field); !ok {"
// arnes: a="if _, ok := fmDocScalar(mapping, field); ok {"
func TestUpdateFrontmatterGuards(t *testing.T) {
	base := strings.Join([]string{"---", "id: t1", "type: task-spec", "status: draft", "tags:", "  - a", "---", "cuerpo"}, "\n")

	t.Run("no renombra id", func(t *testing.T) {
		_, err := UpdateFrontmatterText(base, FrontmatterEdit{Set: map[string]string{FieldID: "otro"}})
		if !errors.Is(err, errCannotRename) {
			t.Errorf("set id debía dar errCannotRename, dio: %v", err)
		}
	})

	t.Run("no edita una lista con set", func(t *testing.T) {
		_, err := UpdateFrontmatterText(base, FrontmatterEdit{Set: map[string]string{FieldTags: "x"}})
		if err == nil || !strings.Contains(err.Error(), "addList/removeList") {
			t.Errorf("set de un campo lista debía redirigir a addList/removeList, dio: %v", err)
		}
	})

	t.Run("no borra la identidad", func(t *testing.T) {
		for _, campo := range []string{FieldID, FieldType} {
			_, err := UpdateFrontmatterText(base, FrontmatterEdit{Remove: []string{campo}})
			if err == nil || !strings.Contains(err.Error(), "protected") {
				t.Errorf("remove %q debía dar error de campo protegido, dio: %v", campo, err)
			}
		}
	})

	t.Run("no deja el archivo sin type válido", func(t *testing.T) {
		// set type:"" pasa las guardas tempranas (no es id, no es lista) pero deja
		// el type vacío: la guarda final tiene que atraparlo.
		_, err := UpdateFrontmatterText(base, FrontmatterEdit{Set: map[string]string{FieldType: ""}})
		if !errors.Is(err, errLeavesInvalid) {
			t.Errorf("set type:\"\" debía dar errLeavesInvalid, dio: %v", err)
		}
	})
}

// TestUpdateFrontmatterErroresDeEntrada fija los dos rechazos de entrada: sin
// frontmatter no hay nada que editar, y un frontmatter que no es un mapa válido
// (YAML roto o bloque vacío) tampoco se edita.
func TestUpdateFrontmatterErroresDeEntrada(t *testing.T) {
	casos := []struct {
		nombre string
		in     string
		want   error
	}{
		{"sin fence", "no hay frontmatter\nsólo prosa", errNoFrontmatter},
		{"YAML roto", "---\nid: [sin cerrar\n---\ncuerpo", errInvalidYAML},
		{"frontmatter vacío", "---\n---\ncuerpo", errInvalidYAML},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := UpdateFrontmatterText(c.in, FrontmatterEdit{Set: map[string]string{FieldStatus: "active"}}); !errors.Is(err, c.want) {
				t.Errorf("dio %v, quería %v", err, c.want)
			}
		})
	}
}

// TestUpdateFrontmatterListas fija AddList (union-dedup, existentes primero) y
// RemoveList (filtra; y si vacía la lista, borra la clave entera).
func TestUpdateFrontmatterListas(t *testing.T) {
	base := strings.Join([]string{"---", "id: t1", "type: task-spec", "depends-on: [goal-1]", "tags: [a, b]", "---", "cuerpo"}, "\n")

	t.Run("AddList hace union-dedup con los existentes primero", func(t *testing.T) {
		out, err := UpdateFrontmatterText(base, FrontmatterEdit{AddList: map[string][]string{FieldDependsOn: {"goal-1", "mod-2"}}})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		got := List(ParseFile(out).Frontmatter, FieldDependsOn)
		if strings.Join(got, ",") != "goal-1,mod-2" {
			t.Errorf("depends-on = %v; quería [goal-1 mod-2] (sin duplicar goal-1)", got)
		}
	})

	t.Run("RemoveList filtra", func(t *testing.T) {
		out, err := UpdateFrontmatterText(base, FrontmatterEdit{RemoveList: map[string][]string{FieldTags: {"a"}}})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		got := List(ParseFile(out).Frontmatter, FieldTags)
		if strings.Join(got, ",") != "b" {
			t.Errorf("tags = %v; quería [b]", got)
		}
	})

	t.Run("RemoveList que vacía borra la clave", func(t *testing.T) {
		out, err := UpdateFrontmatterText(base, FrontmatterEdit{RemoveList: map[string][]string{FieldDependsOn: {"goal-1"}}})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if _, existe := ParseFile(out).Frontmatter[FieldDependsOn]; existe {
			t.Errorf("depends-on quedó vacía pero la clave sigue presente:\n%s", out)
		}
	})
}

// TestUpdateFrontmatterRechazaAnchorRoto fija R5 por el lado que ninguna otra
// prueba cubre: una edición no puede dejar el archivo con YAML ROTO. fmSetScalar
// re-infiere el escalar y limpia el Anchor del nodo que toca; si ese nodo era un
// &b que otra clave aliasaba con *b, el frontmatter re-emitido queda con un *b sin
// su &b y ya NO parsea (medido: "unknown anchor 'b' referenced"). La guarda
// re-parsea lo emitido y rechaza la edición antes de escribir la bomba. Sin la
// guarda, UpdateFrontmatterText devuelve ese YAML roto con err=nil y el archivo
// en disco queda corrupto.
//
// Sabotaje que lo pone en ROJO: anular el control que re-parsea lo emitido, con
// lo que el YAML roto se devuelve con err=nil en vez de rechazarse.
// arnes: archivo="internal/specgraph/frontmatter_edit.go"
// arnes: de="if yaml.Unmarshal([]byte(rendered), &check) != nil {"
// arnes: a="if yaml.Unmarshal([]byte(rendered), &check) != nil && false {"
func TestUpdateFrontmatterRechazaAnchorRoto(t *testing.T) {
	// base define el anchor &b; title lo aliasa con *b. Un set sobre base limpia
	// su anchor y deja el *b colgado; el YAML re-emitido ya no parsea.
	in := strings.Join([]string{"---", "id: foo", "type: task-spec", "base: &b compartido", "title: *b", "---", "cuerpo"}, "\n")

	_, err := UpdateFrontmatterText(in, FrontmatterEdit{Set: map[string]string{"base": "cambiado"}})
	if !errors.Is(err, errAnchorAliasUnsupported) {
		t.Fatalf("set sobre una clave anclada debía dar errAnchorAliasUnsupported, dio: %v", err)
	}
}
