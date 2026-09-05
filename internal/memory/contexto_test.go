package memory

// Pruebas de las lecturas que la FLOTA le hace a la MEMORIA (fase 5 · S14).
//
// Viven acá y no en la tool porque lo que custodian es de almacenamiento —el formato de fecha— y
// porque acá se puede FIJAR el `created_at` de una fila. Con la hora puesta a mano la prueba deja
// de depender de a qué hora del día corra, que es lo que la hacía pasar por el motivo equivocado.

import (
	"context"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// LAS FECHAS DE LA MEMORIA NO ESTÁN EN RFC3339, y comparar con el formato equivocado NO da error:
// da VACÍO. Un vacío acá se lee como «no había nada escrito ese día».
//
// LA VENTANA ES DEL MISMO DÍA A PROPÓSITO. Con una ventana de 24 h las dos formas coinciden —el
// texto `2026-05-05 12:00:00` y `2026-05-05T11:00:00Z` se ordenan por la FECHA, que domina— y el
// sabotaje no rompe nada. Fue exactamente lo que pasó al ejecutarlo: la primera versión de esta
// prueba estaba verde por el motivo equivocado. Dentro del mismo día manda la hora, y ahí el
// espacio contra la `T` decide (0x20 < 0x54): la fila queda del lado de afuera.
//
// Sabotaje: `const formatoDeMemoria = time.RFC3339` → falla acá (y NO falla con una ventana de 24 h).
func TestLaVentanaDeMemoriaUsaElFormatoDeSQLiteYNoRFC3339(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTypedFrom("infra", "", "obs-fija", "infra/tema",
		"MARCAFIJA", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	// La hora se FIJA: sin esto la prueba depende de a qué hora del día corra, y una prueba que
	// sólo falla entre la medianoche y la una es peor que ninguna.
	const cuando = "2026-05-05 12:00:00"
	if _, err := e.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`, cuando, "obs-fija"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, project_id, updated_at)
		VALUES ('infra/fijo.go','g','','h',1,'infra',?)`, cuando); err != nil {
		t.Fatal(err)
	}

	// Una ventana de DOS HORAS alrededor, el mismo día.
	desde := time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)
	v := fleet.Ventana{Desde: desde, Hasta: desde.Add(2 * time.Hour)}
	ctx := context.Background()

	obs, err := e.ObservacionesEnVentana(ctx, v, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].ID != "obs-fija" {
		t.Fatalf("la observación de las 12:00 no cayó en la ventana 11:00–13:00 del mismo día: %v", obs)
	}
	archivos, err := e.CodigoTocadoEnVentana(ctx, v, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) != 1 || archivos[0].Path != "infra/fijo.go" {
		t.Fatalf("el archivo de las 12:00 no cayó en la ventana: %v", archivos)
	}
	if archivos[0].Cuando.IsZero() {
		t.Error("la fecha del archivo no se pudo parsear con el formato de la memoria")
	}

	// CONTROL NEGATIVO: una ventana del mismo día que NO lo contiene tiene que devolver vacío.
	// Sin esto, una consulta que ignorara la ventana pasaría las aserciones de arriba.
	fuera := fleet.Ventana{Desde: desde.Add(4 * time.Hour), Hasta: desde.Add(6 * time.Hour)}
	if obs, err := e.ObservacionesEnVentana(ctx, fuera, 20); err != nil || len(obs) != 0 {
		t.Errorf("una ventana que no contiene la fila devolvió %d observaciones (err=%v)", len(obs), err)
	}
}

// LA MURALLA 2 VALE TAMBIÉN POR ESTA PUERTA: una observación archivada, superseded o en cuarentena
// NO puede aparecer en el contexto de una máquina.
//
// El predicado canónico (`visibleObsPredicate`) lo cumplen nueve consultas de recall. Ésta es la
// décima, y es la primera que llega desde el plano de FLOTA — o sea desde afuera del recall, que
// es exactamente por donde una muralla se rodea sin querer.
//
// Sabotaje: sacar `visibleObsPredicate` del WHERE → falla acá. (Lo encontré ejecutándolo: la
// primera versión de estas pruebas no sembraba ninguna fila tapada, así que ese sabotaje quedaba
// verde y la muralla estaba sin custodiar por este lado.)
func TestElContextoNoTraeObservacionesTapadas(t *testing.T) {
	e := newTestEngine(t)
	const cuando = "2026-05-05 12:00:00"
	sembrar := func(id, marca string) {
		t.Helper()
		if err := e.SaveObservationTypedFrom("infra", "", id, "infra/tema", marca, 1.0, "semantic", "local", nil); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`, cuando, id); err != nil {
			t.Fatal(err)
		}
	}
	sembrar("obs-viva", "MARCAVIVA")
	sembrar("obs-archivada", "MARCAARCHIVADA")
	sembrar("obs-cuarentena", "MARCACUARENTENA")
	sembrar("obs-superseded", "MARCASUPERSEDED")
	for sql, id := range map[string]string{
		`UPDATE observations SET archived = 1 WHERE id = ?`:               "obs-archivada",
		`UPDATE observations SET quarantined = 1 WHERE id = ?`:            "obs-cuarentena",
		`UPDATE observations SET superseded_by = 'obs-viva' WHERE id = ?`: "obs-superseded",
	} {
		if _, err := e.db.Exec(sql, id); err != nil {
			t.Fatal(err)
		}
	}
	desde := time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)
	obs, err := e.ObservacionesEnVentana(context.Background(),
		fleet.Ventana{Desde: desde, Hasta: desde.Add(2 * time.Hour)}, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].ID != "obs-viva" {
		ids := make([]string, 0, len(obs))
		for _, o := range obs {
			ids = append(ids, o.ID)
		}
		t.Fatalf("la ventana trajo %v; sólo tenía que traer obs-viva", ids)
	}
}

// El aislamiento por proyecto de las DOS lecturas nuevas. Es la misma muralla del recall, por una
// puerta nueva: sin el scope, la flota se convierte en un camino lateral a la memoria ajena.
//
// Sabotaje: sacar `scopeClause` de ObservacionesEnVentana, o el bloque de scope de
// CodigoTocadoEnVentana → falla acá.
func TestLasLecturasDeContextoRespetanElProyecto(t *testing.T) {
	e := newTestEngine(t)
	const cuando = "2026-05-05 12:00:00"
	for _, p := range []string{"infra", "otro"} {
		id := "obs-" + p
		if err := e.SaveObservationTypedFrom(p, "", id, p+"/tema", "MARCA-"+p, 1.0, "semantic", "local", nil); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`, cuando, id); err != nil {
			t.Fatal(err)
		}
		if _, err := e.db.Exec(`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, project_id, updated_at)
			VALUES (?,'g','','h',1,?,?)`, p+"/x.go", p, cuando); err != nil {
			t.Fatal(err)
		}
	}
	desde := time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)
	v := fleet.Ventana{Desde: desde, Hasta: desde.Add(2 * time.Hour)}
	ctx := WithProjectScope(context.Background(), ProjectScope{ProjectID: "infra"})

	obs, err := e.ObservacionesEnVentana(ctx, v, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range obs {
		if o.ID == "obs-otro" {
			t.Errorf("FUGA cross-tenant: la ventana de `infra` trajo la observación de `otro`")
		}
	}
	if len(obs) != 1 {
		t.Errorf("esperaba SÓLO la de infra, vinieron %d: %v", len(obs), obs)
	}
	archivos, err := e.CodigoTocadoEnVentana(ctx, v, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range archivos {
		if a.Path == "otro/x.go" {
			t.Error("FUGA cross-tenant: el código de `otro` apareció en la ventana de `infra`")
		}
	}
	if len(archivos) != 1 {
		t.Errorf("esperaba SÓLO el archivo de infra, vinieron %d: %v", len(archivos), archivos)
	}
}

// Una ventana inválida da ERROR y no barre la tabla. Fail-closed, igual que la cronología.
func TestLasLecturasDeContextoRechazanUnaVentanaInvalida(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	if _, err := e.ObservacionesEnVentana(ctx, fleet.Ventana{}, 20); err == nil {
		t.Error("una ventana vacía tiene que dar error en ObservacionesEnVentana")
	}
	if _, err := e.CodigoTocadoEnVentana(ctx, fleet.Ventana{}, 20); err == nil {
		t.Error("una ventana vacía tiene que dar error en CodigoTocadoEnVentana")
	}
}

// LAS DOS SUPOSICIONES SOBRE EL DRIVER, CLAVADAS (A61).
//
// Toda la contención de A61 se apoya en una asimetría de `modernc.org/sqlite`: convierte al LEER y
// no al COMPARAR. Si esa asimetría cambia —una actualización del driver, un cambio de tipo
// declarado— el síntoma NO es un error: es una ventana que devuelve vacío, y un vacío se lee como
// «no había nada escrito ese día». Esta prueba existe para que ese cambio se entere acá y no en
// producción seis meses después.
//
// Se miden las dos mitades por separado y con DOS filas, porque cada una necesita algo distinto:
// la primera necesita que la fecha la escriba SQLITE (es lo que se está midiendo) y la segunda
// necesita una hora FIJA (con una ventana que cruce la medianoche, las dos formas coinciden porque
// manda la fecha, y la prueba pasaría por el motivo equivocado — ya pasó en este archivo).
//
// Sabotajes: `const formatoDeMemoria = time.RFC3339` rompe la segunda mitad; declarar la columna
// como TEXT en vez de DATETIME rompe la primera.
func TestElDriverConvierteAlLeerYNoAlComparar(t *testing.T) {
	e := newTestEngine(t)

	// ── MITAD 1: qué escribe SQLite, y qué devuelve el driver ────────────────────────────────
	if err := e.SaveObservationTypedFrom("infra", "", "obs-driver", "infra/tema",
		"MARCADRIVER", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}

	// Los BYTES GUARDADOS, preguntados DENTRO de SQLite para que el driver no pueda entrometerse.
	// El GLOB exige el separador ESPACIO: si algún día se guardara con `T`, esto lo dice.
	var conFormatoDeSQLite int
	if err := e.db.QueryRow(
		`SELECT count(*) FROM observations WHERE id = ? AND created_at GLOB '????-??-?? ??:??:??'`,
		"obs-driver").Scan(&conFormatoDeSQLite); err != nil {
		t.Fatal(err)
	}
	if conFormatoDeSQLite != 1 {
		t.Error("CURRENT_TIMESTAMP dejó de escribir `YYYY-MM-DD HH:MM:SS`: el WHERE de todas las ventanas de memoria compara contra ese formato")
	}

	// Y lo que RECIBE Go de la misma columna: RFC3339, aunque los bytes de arriba no lo sean.
	// Ésa es la trampa entera — mirar esto lleva a la conclusión equivocada sobre cómo comparar.
	var leido string
	if err := e.db.QueryRow(`SELECT created_at FROM observations WHERE id = ?`, "obs-driver").Scan(&leido); err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, leido); err != nil {
		t.Errorf("el driver dejó de convertir al leer (devolvió %q): el parseo dual de parseFechaDeMemoria era lo que sostenía esto", leido)
	}

	// ── MITAD 2: el WHERE compara los bytes crudos, y el formato equivocado NO da error ───────
	//
	// Hora FIJA y ventana del MISMO DÍA: es lo único que hace que decida la hora y no la fecha.
	const cuando = "2026-05-05 12:00:00"
	if err := e.SaveObservationTypedFrom("infra", "", "obs-comparar", "infra/tema",
		"MARCACOMPARAR", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`, cuando, "obs-comparar"); err != nil {
		t.Fatal(err)
	}
	corte := time.Date(2026, 5, 5, 11, 0, 0, 0, time.UTC)

	var conBueno, conMalo int
	if err := e.db.QueryRow(`SELECT count(*) FROM observations WHERE id = ? AND created_at >= ?`,
		"obs-comparar", corte.Format(formatoDeMemoria)).Scan(&conBueno); err != nil {
		t.Fatal(err)
	}
	if err := e.db.QueryRow(`SELECT count(*) FROM observations WHERE id = ? AND created_at >= ?`,
		"obs-comparar", corte.Format(time.RFC3339)).Scan(&conMalo); err != nil {
		t.Fatal(err)
	}
	if conBueno != 1 {
		t.Error("el formato de SQLite dejó de encontrar una fila que SÍ está dentro de la ventana")
	}
	// Lo que se afirma acá no es «RFC3339 está mal»: es que estar mal NO SE NOTA. Devuelve cero
	// filas y ningún error, y ese cero se lee como «no había nada escrito ese día».
	if conMalo != 0 {
		t.Error("comparar con RFC3339 encontró la fila: si eso cambia, el motivo por el que existe formatoDeMemoria dejó de valer y hay que revisar las ventanas")
	}
}
