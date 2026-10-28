package mcp

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// Fase 2 de id-corto en el LÍMITE DEL TOOL: los consumidores por-PK del id aprenden a tratar la ref
// corta de 8 hex que hoy emite el envoltorio del hook (memory.IDCorto). discard la RESUELVE —archivar
// es sobre la fila que el agente identificó—; save la RECHAZA —un UPSERT por prefijo convertiría un
// alta en sobrescritura de otra fila—. El resolvedor
// estricto y su aislamiento están cubiertos por TestResolverRefCortaCtx; acá se fija el CABLEADO.

// TestDiscardResuelveSupersededByCorto es el caso VIVO: el subagente de tareas saca de la cuarentena
// una propuesta (id completo del board) por estar superada por una nota que vio en el recall, y ese id
// llega ACORTADO. Sin resolverlo, el COUNT por PK exacta de DescartarPropuesta no matchea y la
// propuesta rancia queda viva (superseded_by jamás se setea) — el modo «la memoria empieza a fallar».
//
// Sabotaje: no resolver la ref corta de `reemplazadaPor` en DescartarPropuestaCtx ⇒ el id corto no
// matchea ninguna fila ⇒ descartar devuelve «no encontrada» ⇒ ROJO.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="\tif reemplazadaPor, err = e.ResolverRefCortaCtx(ctx, reemplazadaPor); err != nil {\n\t\treturn err\n\t}\n"
// arnes: a=""
// arnes: prueba="TestDiscardResuelveSupersededByCorto"
func TestDiscardResuelveSupersededByCorto(t *testing.T) {
	dir := t.TempDir()
	engine := memtest.NuevoEngine(t, dir)
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	proponer := func(topic, contenido string) string {
		t.Helper()
		out, e := call(t, s, "musubi_propose_observation", map[string]interface{}{"topic_key": topic, "content": contenido})
		if e != nil {
			t.Fatalf("proponer: %+v", e)
		}
		return idDeLaPropuesta(t, out)
	}
	vieja := proponer("t/estado", "estado viejo del trabajo de prueba")
	nueva := proponer("t/estado", "estado nuevo del trabajo de prueba")

	// El agente devuelve el id de `nueva` ACORTADO, como lo emite el envoltorio del hook (IDCorto). Da
	// igual que `nueva` sea una propuesta en cuarentena y no una nota visible: discard sólo exige que
	// `superseded_by` EXISTA, y lo que se prueba es que el corto se resuelva al id completo.
	corto := memory.IDCorto(nueva)
	if len(corto) != 8 {
		t.Fatalf("IDCorto(%s)=%q no es una ref corta de 8 hex (¿cambió el emisor?)", nueva, corto)
	}

	if _, e := call(t, s, "musubi_discard_proposal", map[string]interface{}{"id": vieja, "superseded_by": corto}); e != nil {
		t.Fatalf("descartar con superseded_by corto %q: %+v (¿no se resolvió al id completo?)", corto, e)
	}

	// superseded_by quedó apuntando al id COMPLETO, no al corto: la resolución ocurrió. Se lee de la
	// base en sólo lectura, como el test hermano de descartar.
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, config.DirName, config.DBFile)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sup string
	if err := db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id=?`, vieja).Scan(&sup); err != nil {
		t.Fatal(err)
	}
	if sup != nueva {
		t.Errorf("superseded_by de la descartada = %q; quería el id completo %q (la ref corta no se resolvió)", sup, nueva)
	}
}

// TestSaveRechazaIdCortoNoDuplica fija que save_observation con id= RECHAZA una ref corta. Un save
// con-id es un UPSERT: resolver el corto convertiría un alta en sobrescritura de otra fila, y NO
// resolverlo insertaría un duplicado silencioso de 8 hex que el ON CONFLICT(id) no ve. Las dos rompen
// «cero pérdida», así que se rechaza y se avisa. Los dos asertos —error visible Y conteo
// que no crece— cierran las dos mitades: el rechazo ocurre y NO deja una fila espuria.
//
// Sabotaje: invertir la condición del rechazo (que sólo rechace lo que NO es ref corta) ⇒ el id corto
// pasa al UPSERT ⇒ no hay error y aparece una fila nueva ⇒ ROJO por los dos asertos.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="memory.EsRefCorta(sid)"
// arnes: a="!memory.EsRefCorta(sid)"
// arnes: prueba="TestSaveRechazaIdCortoNoDuplica"
func TestSaveRechazaIdCortoNoDuplica(t *testing.T) {
	engine := memtest.NuevoEngine(t, t.TempDir())
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	antes, err := engine.Insights()
	if err != nil {
		t.Fatalf("insights antes: %v", err)
	}

	out, e := call(t, s, "musubi_save_observation", map[string]interface{}{
		"id": "abcd1234", "topic_key": "t/x", "content": "intento de guardar por ref corta",
	})
	if e == nil {
		t.Fatalf("save con id ref-corta devolvió éxito (%q); tenía que rechazar", textOf(t, out))
	}
	if e.Code != codeInvalidParams {
		t.Errorf("save con id ref-corta: code=%d; quería codeInvalidParams (%d)", e.Code, codeInvalidParams)
	}
	// El aviso tiene que orientar al id completo: nombrar la ref corta y la vía de expansión.
	if !strings.Contains(e.Message, "ref corta") || !strings.Contains(e.Message, "memory_expand") {
		t.Errorf("el aviso no orienta al id completo: %q", e.Message)
	}

	despues, err := engine.Insights()
	if err != nil {
		t.Fatalf("insights después: %v", err)
	}
	if despues.Observations.Total != antes.Observations.Total {
		t.Errorf("el save rechazado creó una fila: total %d → %d (duplicado silencioso de 8 hex)",
			antes.Observations.Total, despues.Observations.Total)
	}
}

// TestDiscardResuelveIdCorto cubre la OTRA mitad de la resolución de discard: no sólo `superseded_by`,
// también el `id` de la propia propuesta a archivar puede llegar CORTO. DescartarPropuestaCtx resuelve
// las dos (líneas separadas), pero TestDiscardResuelveSupersededByCorto sólo ejercita la segunda; sin
// esto, una mutación que borre la resolución del `id` queda VERDE. Es defensivo —en uso real el id de
// la propuesta sale de musubi_work, completo— pero el código lo resuelve, así que se mide.
//
// Sabotaje: no resolver la ref corta de `id` en DescartarPropuestaCtx ⇒ el COUNT por PK exacta de
// DescartarPropuesta no matchea el corto ⇒ descartar devuelve «no encontrada» ⇒ ROJO.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="\tif id, err = e.ResolverRefCortaCtx(ctx, id); err != nil {\n\t\treturn err\n\t}\n"
// arnes: a=""
// arnes: prueba="TestDiscardResuelveIdCorto"
func TestDiscardResuelveIdCorto(t *testing.T) {
	dir := t.TempDir()
	engine := memtest.NuevoEngine(t, dir)
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	out, e := call(t, s, "musubi_propose_observation", map[string]interface{}{
		"topic_key": "t/estado", "content": "propuesta a descartar por su id corto",
	})
	if e != nil {
		t.Fatalf("proponer: %+v", e)
	}
	full := idDeLaPropuesta(t, out)

	// El agente devuelve el id de la propuesta ACORTADO, como lo emite el envoltorio del hook.
	corto := memory.IDCorto(full)
	if len(corto) != 8 {
		t.Fatalf("IDCorto(%s)=%q no es una ref corta de 8 hex (¿cambió el emisor?)", full, corto)
	}

	// Descartar por el id CORTO, sin superseded_by: el único corto en juego es el `id`.
	if _, e := call(t, s, "musubi_discard_proposal", map[string]interface{}{"id": corto}); e != nil {
		t.Fatalf("descartar con id corto %q: %+v (¿no se resolvió al id completo?)", corto, e)
	}

	// La fila del id COMPLETO quedó archivada: la resolución del corto ocurrió y actuó sobre la correcta.
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, config.DirName, config.DBFile)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var archived int
	if err := db.QueryRow(`SELECT archived FROM observations WHERE id=?`, full).Scan(&archived); err != nil {
		t.Fatal(err)
	}
	if archived != 1 {
		t.Errorf("la propuesta no quedó archivada (archived=%d): el id corto no se resolvió a la fila", archived)
	}
}
