package memory

import (
	"context"
	"errors"
	"testing"
)

// insObsCruda siembra una observación con id y project_id CONTROLADOS por INSERT directo (el mismo
// patrón que las pruebas de migración y de FTS). Es la única forma de fijar el prefijo de 8 hex a
// voluntad —para forzar una colisión— y de atribuir la fila a un tenant preciso —para el aislamiento—.
// Las columnas que no se dan caen a su default del esquema (scope='local', quarantined=0, archived=0).
func insObsCruda(t *testing.T, e *DbEngine, id, projectID, content string) {
	t.Helper()
	if _, err := e.db.Exec(
		`INSERT INTO observations (id, topic_key, content, project_id) VALUES (?,?,?,?)`,
		id, "t/resolver", content, projectID); err != nil {
		t.Fatalf("insObsCruda %s: %v", id, err)
	}
}

// TestResolverRefCortaCtx fija el contrato de ResolverRefCortaCtx (fase 2 de id-corto): traduce una ref
// corta de 8 hex al id completo, ESTRICTO (una fila o error) y acotado al scope. Es la mitad que la
// fase 1 dejó sólo en la hidratación; sin ella, los consumidores por-PK (promote/discard) no matchean
// el id corto que hoy emite el envoltorio del hook. A diferencia de hydrateByIDs, acá la ambigüedad es
// un error —no el superconjunto—, y el aislamiento Track 17 se hereda de scopeClause: el scope FILTRA
// ANTES de juzgar la ambigüedad, así que una colisión de prefijo entre tenants no bloquea al dueño.
//
// DOMINIO = RANGO. Dos casos fijan que el match se restringe a filas con id canónico: IDCorto sólo
// acorta UUID canónicos, así que un id no-canónico que comparta el prefijo de 8 hex NO es un match de
// esta ref. Sin el filtro, «eeee5555» se vuelve ambiguo por el colisor y «ffff6666» resolvería a una
// fila que el emisor jamás pudo nombrar.
//
// Sabotaje: quitar el filtro de canonicidad del scan ⇒ el colisor no-canónico entra al match ⇒ ROJO
// por los dos casos de arriba.
// arnes: archivo="internal/memory/resolver_ref_corta.go"
// arnes: de="\t\tif !esUUIDCanonico(full) {\n\t\t\tcontinue\n\t\t}\n"
// arnes: a=""
// arnes: prueba="TestResolverRefCortaCtx"
func TestResolverRefCortaCtx(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { e.Close() })

	const (
		unicaP1     = "aaaa1111-0000-4000-8000-000000000001" // prefijo aaaa1111 único, tenant p1
		colA        = "bbbb2222-0000-4000-8000-000000000001" // prefijo bbbb2222 …
		colB        = "bbbb2222-0000-4000-8000-000000000002" // … colisiona, ambos p1 ⇒ ambiguo
		ajenaP2     = "cccc3333-0000-4000-8000-000000000001" // prefijo cccc3333, tenant p2 (ajeno a p1)
		crossP1     = "dddd4444-0000-4000-8000-000000000001" // prefijo dddd4444 en p1 …
		crossP2     = "dddd4444-0000-4000-8000-000000000002" // … y en p2: el scope decide antes que la ambigüedad
		canonE      = "eeee5555-0000-4000-8000-000000000001" // prefijo eeee5555 CANÓNICO en p1 …
		colNoCanon  = "eeee5555aaaa0000"                     // … con un COLISOR no-canónico (16 hex, mismo prefijo): se ignora
		soloNoCanon = "ffff6666deadbeef"                     // prefijo ffff6666 SÓLO no-canónico (sin hermano canónico)
	)
	insObsCruda(t, e, unicaP1, "p1", "unica p1")
	insObsCruda(t, e, colA, "p1", "colision A")
	insObsCruda(t, e, colB, "p1", "colision B")
	insObsCruda(t, e, ajenaP2, "p2", "ajena de p2")
	insObsCruda(t, e, crossP1, "p1", "cross p1")
	insObsCruda(t, e, crossP2, "p2", "cross p2")
	insObsCruda(t, e, canonE, "p1", "canonica con colisor no-canonico")
	insObsCruda(t, e, colNoCanon, "p1", "colisor no-canonico de 16 hex")
	insObsCruda(t, e, soloNoCanon, "p1", "solo colisor no-canonico, sin hermano canonico")

	ctxP1 := WithProjectScope(context.Background(), ProjectScope{ProjectID: "p1"})
	ctxP2 := WithProjectScope(context.Background(), ProjectScope{ProjectID: "p2"})
	ctxFed := context.Background() // sin scope ⇒ federado (sin filtro de tenant)

	casos := []struct {
		nombre  string
		ctx     context.Context
		in      string
		wantID  string // sólo se compara cuando wantErr es nil
		wantErr error  // nil ⇒ sin error
	}{
		{"corta única resuelve al id completo", ctxP1, "aaaa1111", unicaP1, nil},
		{"corta ambigua (dos del mismo tenant) es error", ctxP1, "bbbb2222", "", ErrRefCortaAmbigua},
		{"corta ajena NO resuelve bajo el scope propio", ctxP1, "cccc3333", "", ErrObservationNotFound},
		{"corta inexistente no resuelve", ctxP1, "deadbeef", "", ErrObservationNotFound},
		{"el scope filtra ANTES de la ambigüedad: p1 ve una sola", ctxP1, "dddd4444", crossP1, nil},
		{"el scope filtra ANTES de la ambigüedad: p2 ve la otra", ctxP2, "dddd4444", crossP2, nil},
		{"federado ve las dos ⇒ dddd4444 vuelve a ser ambiguo", ctxFed, "dddd4444", "", ErrRefCortaAmbigua},
		{"federado resuelve la ajena (sin filtro de tenant)", ctxFed, "cccc3333", ajenaP2, nil},
		{"la ajena resuelve bajo SU propio scope", ctxP2, "cccc3333", ajenaP2, nil},
		{"colisor no-canónico se ignora: la ref resuelve a la fila canónica", ctxP1, "eeee5555", canonE, nil},
		{"sólo un colisor no-canónico ⇒ no resuelve (dominio = rango del emisor)", ctxP1, "ffff6666", "", ErrObservationNotFound},
		{"id completo pasa VERBATIM (sin tocar la base)", ctxP1, unicaP1, unicaP1, nil},
		{"id completo inexistente pasa verbatim (lo decide el PK de arriba)", ctxP1, "99999999-0000-4000-8000-000000000009", "99999999-0000-4000-8000-000000000009", nil},
		{"vacío pasa verbatim", ctxP1, "", "", nil},
		{"forma no-corta no-uuid pasa verbatim", ctxP1, "commit-abc", "commit-abc", nil},
	}
	for _, c := range casos {
		got, err := e.ResolverRefCortaCtx(c.ctx, c.in)
		if c.wantErr != nil {
			if !errors.Is(err, c.wantErr) {
				t.Errorf("%s: err=%v; quería %v", c.nombre, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: err inesperado %v", c.nombre, err)
			continue
		}
		if got != c.wantID {
			t.Errorf("%s: resolvió %q; quería %q", c.nombre, got, c.wantID)
		}
	}
}

// TestPromoteObservationCtxResuelveCorto fija que PromoteObservationCtx RESUELVE una ref corta antes
// del lookup por PK (el spec nombra a este método del engine, no al handler). Sin la resolución,
// promover por el id corto que emite el hook daría ErrObservationNotFound y la promoción se perdería
// en silencio. El assert sobre `scope` prueba que actuó sobre la fila CORRECTA, no sobre otra.
//
// Sabotaje: quitar la resolución de la ref corta en PromoteObservationCtx ⇒ promover por el corto no
// encuentra la fila ⇒ ROJO.
// arnes: archivo="internal/memory/scope.go"
// arnes: de="\tid, err := e.ResolverRefCortaCtx(ctx, id)\n\tif err != nil {\n\t\treturn err\n\t}\n"
// arnes: a=""
// arnes: prueba="TestPromoteObservationCtxResuelveCorto"
func TestPromoteObservationCtxResuelveCorto(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { e.Close() })

	const full = "feedbeef-0000-4000-8000-000000000001" // canónico ⇒ IDCorto lo acorta a "feedbeef"
	insObsCruda(t, e, full, "p1", "una nota visible para promover")

	corto := IDCorto(full)
	if corto != "feedbeef" || len(corto) != 8 {
		t.Fatalf("IDCorto(%s)=%q; el emisor y el resolvedor tienen que coincidir en el prefijo", full, corto)
	}

	ctx := WithProjectScope(context.Background(), ProjectScope{ProjectID: "p1"})
	if err := e.PromoteObservationCtx(ctx, corto); err != nil {
		t.Fatalf("promover por la ref corta %q falló: %v (¿no se resolvió al id completo?)", corto, err)
	}
	// Promovió la fila CORRECTA: la del id completo quedó 'shared'. Si hubiera actuado sobre otra —o
	// sobre ninguna— este scope seguiría 'local'.
	var scope string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id=?`, full).Scan(&scope); err != nil {
		t.Fatalf("leer scope de %s: %v", full, err)
	}
	if scope != ScopeShared {
		t.Errorf("tras promover por el corto, scope de %s = %q; quería %q", full, scope, ScopeShared)
	}
}
