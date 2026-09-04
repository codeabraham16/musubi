package memory

import (
	"errors"
	"strings"
	"testing"
)

// LAS TRES FORMAS SON LAS MEDIDAS, no inventadas: salen de contar las 4502 observaciones del
// cerebro central el 2026-09-04 (36 · 29 · 8). Y el cuarto caso es el que impide que la guarda
// sea un candado: una observación que DOCUMENTE este defecto tiene que poder citar la etiqueta.
func TestSobreDeLlamadaComidoReconoceLasTresFormasYNoLaMencion(t *testing.T) {
	casos := []struct {
		nombre  string
		content string
		comido  bool
	}{
		{
			// 36 de 73: el content termina en el cierre y nada más.
			nombre:  "forma 1 · termina en </content>",
			content: "…adoptar caveman directo. Pendiente de confirmación del usuario.</content>",
			comido:  true,
		},
		{
			// 29 de 73: el sobre entero.
			nombre: "forma 2 · llega hasta </invoke>",
			content: "…el mismo trabajo que cubre la skill lc-portar-64bit.</content>\n" +
				"<mem_type>semantic</mem_type>\n<importance>1.6</importance>\n</invoke>",
			comido: true,
		},
		{
			// 8 de 73: cortado en medio del sobre.
			nombre: "forma 3 · corta en medio del sobre",
			content: "…usó violeta+sombras y quedó off-brand en los específicos.</content>\n" +
				"<parameter name=\"importance\">1.6",
			comido: true,
		},
		{
			// EL CASO QUE IMPIDE EL CANDADO. Lo que distingue el bug de la cita es qué viene
			// DESPUÉS: sobre en uno, prosa en el otro.
			nombre: "mención legítima · hay prosa después",
			content: "El defecto es que el content termina en `</content>` y todo lo que sigue " +
				"es el sobre de la llamada. Se mide contando las observaciones que lo tienen.",
			comido: false,
		},
		{
			nombre:  "sin la etiqueta · no se toca",
			content: "Una observación normal, con `<` y `>` sueltos: a < b y b > c.",
			comido:  false,
		},
		{
			// Una cita CON sobre después seguiría siendo el bug, y está bien: el sobre real es el
			// último, y se corta en el ÚLTIMO </content> justamente por esto.
			nombre: "cita Y sobre · gana el sobre",
			content: "El defecto deja `</content>` adentro y sigue con prosa explicándolo.</content>\n" +
				"</invoke>",
			comido: true,
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			comido, detalle := SobreDeLlamadaComido(c.content)
			if comido != c.comido {
				t.Fatalf("SobreDeLlamadaComido = %v, se esperaba %v (detalle: %q)", comido, c.comido, detalle)
			}
			if comido && detalle == "" {
				t.Error("se detectó el sobre y el detalle vino vacío: el error tiene que poder " +
					"nombrar lo que se tragó, o el llamador reescribe a ciegas")
			}
		})
	}
}

// LAS NUEVE VARIANTES PÚBLICAS RECHAZAN, y no una sola.
//
// Es el riesgo concreto de este arreglo: hay DIEZ formas de guardar una observación y la guarda
// vive en la privada que todas comparten. Si mañana alguien agrega una variante que arma el INSERT
// por su cuenta —como ya hace inboundsync, que está exento a propósito y con su razón escrita—
// esta tabla no la va a nombrar, pero al menos deja medido que hoy ninguna de las nueve se escapa.
//
// Sabotaje que la hace fallar: sacar el `if comido, detalle := ...` del principio de
// saveObservation. Las nueve se ponen en rojo de una vez, que es lo que dice que el punto único
// es único de verdad.
func TestLasNueveVariantesDeGuardadoRechazanElSobre(t *testing.T) {
	const sucio = "algo que alguien quiso guardar.</content>\n<importance>1.9</importance>\n</invoke>"

	variantes := map[string]func(e *DbEngine, id string) error{
		"SaveObservation": func(e *DbEngine, id string) error {
			return e.SaveObservation(id, "t", sucio, nil)
		},
		"SaveObservationWithImportance": func(e *DbEngine, id string) error {
			return e.SaveObservationWithImportance(id, "t", sucio, 1.9, nil)
		},
		"SaveObservationTyped": func(e *DbEngine, id string) error {
			return e.SaveObservationTyped(id, "t", sucio, 1.9, "semantic", "local", nil)
		},
		"SaveObservationTypedFrom": func(e *DbEngine, id string) error {
			return e.SaveObservationTypedFrom("", "", id, "t", sucio, 1.9, "semantic", "local", nil)
		},
		"SaveObservationTypedWithOrigins": func(e *DbEngine, id string) error {
			return e.SaveObservationTypedWithOrigins("", "", id, "t", sucio, 1.9, "semantic", "local", nil, nil)
		},
		"SaveObservationDeduped": func(e *DbEngine, id string) error {
			_, _, err := e.SaveObservationDeduped("t", sucio+id, 1.9, nil)
			return err
		},
		"SaveObservationDedupedTyped": func(e *DbEngine, id string) error {
			_, _, err := e.SaveObservationDedupedTyped("t", sucio+id, 1.9, "semantic", "local", nil)
			return err
		},
		"SaveObservationDedupedTypedFrom": func(e *DbEngine, id string) error {
			_, _, err := e.SaveObservationDedupedTypedFrom("", "", "t", sucio+id, 1.9, "semantic", "local", nil)
			return err
		},
		"SaveObservationDedupedTypedFromWithOrigins": func(e *DbEngine, id string) error {
			_, _, err := e.SaveObservationDedupedTypedFromWithOrigins("", "", "t", sucio+id, 1.9, "semantic", "local", nil, nil)
			return err
		},
	}
	// CONTROL DE QUE MIRÓ TODAS: si alguien agrega una variante y no la suma acá, este número deja
	// de cuadrar y hay que decidir a mano — que es el punto.
	if len(variantes) != 9 {
		t.Fatalf("la tabla cubre %d variantes y son 9: sumá la nueva o explicá por qué queda afuera", len(variantes))
	}

	e := newTestEngine(t)
	for nombre, guardar := range variantes {
		t.Run(nombre, func(t *testing.T) {
			err := guardar(e, "obs-"+nombre)
			if err == nil {
				t.Fatalf("%s ACEPTÓ un content con el sobre comido. Los campos del sobre quedan "+
					"como texto y sus columnas en el default: el recall las ordena mal y nada lo dice", nombre)
			}
			if !strings.Contains(err.Error(), "se comió el cierre de su propia llamada") {
				t.Errorf("%s falló, pero por otra razón: %v", nombre, err)
			}
		})
	}
}

// Y UNA OBSERVACIÓN LIMPIA SIGUE GUARDÁNDOSE. Sin esto la guarda podría rechazar todo y las nueve
// pruebas de arriba seguirían en verde: probarían que falla, no que discrimina.
func TestUnaObservacionLimpiaSeGuardaIgual(t *testing.T) {
	e := newTestEngine(t)
	limpia := "Una observación normal que menciona `</content>` en prosa y sigue explicándolo."
	if err := e.SaveObservationTyped("obs-limpia", "t", limpia, 1.9, "semantic", "local", nil); err != nil {
		t.Fatalf("una observación limpia fue rechazada: %v", err)
	}
	var importance float64
	var memType string
	if err := e.db.QueryRow(
		`SELECT importance, COALESCE(mem_type,'') FROM observations WHERE id = ?`, "obs-limpia").
		Scan(&importance, &memType); err != nil {
		t.Fatalf("no se pudo leer la observación guardada: %v", err)
	}
	// Y sus campos llegaron a sus columnas, que es justo lo que el defecto rompía.
	if importance != 1.9 || memType != "semantic" {
		t.Errorf("importance=%v mem_type=%q; se esperaba 1.9 y \"semantic\" — si estos no llegan, "+
			"la guarda no arregló nada", importance, memType)
	}
}

func TestErrSobreDeLlamadaNombraLoQueSeTrago(t *testing.T) {
	err := ErrSobreDeLlamada("el texto termina en `</content>`")
	if err == nil {
		t.Fatal("ErrSobreDeLlamada devolvió nil")
	}
	for _, exigido := range []string{"importance", "mem_type", "origin_paths", "</content>"} {
		if !strings.Contains(err.Error(), exigido) {
			t.Errorf("el mensaje no nombra %q, y el llamador tiene que saber QUÉ perdió: %v", exigido, err)
		}
	}
	if errors.Unwrap(err) != nil {
		t.Log("nota: el error envuelve otro; no es un problema, sólo queda anotado")
	}
}

// EL CHEQUEO DEL DOCTOR VE LAS QUE YA ESTÁN, y distingue las reparables.
//
// La fila sucia se INSERTA A MANO, y no por el camino normal, porque la guarda de saveObservation
// ya no la deja pasar. Es la regla que dejó escrita S1: cuando una guarda es defensa en
// profundidad, la prueba tiene que SIMULAR el error del que protege, no ejercitar el camino feliz —
// si no, prueba que el camino feliz funciona y no que la guarda sirve.
func TestElDoctorVeLasObservacionesConElSobreComido(t *testing.T) {
	e := newTestEngine(t)

	sembrar := func(id, content string, importancia float64) {
		t.Helper()
		if _, err := e.db.Exec(
			`INSERT INTO observations (id, topic_key, content, gist, content_hash, tokens, importance, scope)
			 VALUES (?, ?, ?, '', ?, 0, ?, 'local')`,
			id, "t", content, ContentHash(content), importancia); err != nil {
			t.Fatalf("no se pudo sembrar %s: %v", id, err)
		}
	}

	// Antes de sembrar nada, el chequeo tiene que decir ok — si no, cualquier warning posterior
	// podría venir de otra fila y la prueba estaría midiendo ruido.
	if r := checkSwallowedEnvelope(e); r.Status != "ok" {
		t.Fatalf("con la base limpia el chequeo dio %q: %s", r.Status, r.Message)
	}

	// 1 · fea pero sin nada que devolver: no declara importance en el texto.
	sembrar("sucia-sin-dato", "algo que se guardó mal.</content>", 1.0)
	// 2 · REPARABLE: el texto dice 1.9 y la columna quedó en 1.0, el default.
	sembrar("sucia-reparable", "otra cosa.</content>\n<importance>1.9</importance>\n</invoke>", 1.0)
	// 3 · declara 1.9 y la columna YA dice 1.9: fea, pero su ranking está bien. No es reparable:
	//     no hay nada que devolverle, y contarla infla el número que decide si vale la pena tocar
	//     producción.
	sembrar("sucia-ya-coincide", "y otra.</content>\n<importance>1.9</importance>", 1.9)
	// 4 · CITA la etiqueta en prosa: NO tiene que contarse.
	sembrar("limpia-que-cita", "El bug deja `</content>` adentro y sigue con prosa que lo explica.", 1.0)

	r := checkSwallowedEnvelope(e)
	if r.Status != "warning" {
		t.Fatalf("con tres filas sucias el chequeo dio %q: %s", r.Status, r.Message)
	}
	if !strings.Contains(r.Message, "3 observación(es)") {
		t.Errorf("el chequeo no contó 3 con el sobre comido (la que cita en prosa no va): %s", r.Message)
	}
	if !strings.Contains(r.Message, "1 todavía dicen") {
		t.Errorf("el chequeo no contó 1 reparable — `sucia-ya-coincide` no lo es, porque su columna "+
			"ya tiene el valor declarado: %s", r.Message)
	}
	if r.Repairable {
		t.Error("el chequeo se declaró reparable y no tiene `apply`: reescribir el content cambia " +
			"el content_hash, que es la clave del dedup y viaja en el sync")
	}
}

// Y el registry del doctor lo INCLUYE. Sin esto el chequeo existe y nadie lo corre — que es la
// forma exacta de guarda decorativa que este repo persigue.
func TestElRegistryDelDoctorIncluyeElChequeoDelSobre(t *testing.T) {
	e := newTestEngine(t)
	for _, c := range e.doctorChecks() {
		if c.code == "swallowed_envelope" {
			if !c.deep {
				t.Error("el chequeo recorre el content de las observaciones: tiene que ser `deep`, " +
					"o el diagnóstico rápido paga un scan cada pocos segundos")
			}
			if c.apply != nil {
				t.Error("el chequeo tiene `apply` y no debería: ver el comentario de " +
					"checkSwallowedEnvelope — la reparación no puede devolver lo que nunca se escribió")
			}
			return
		}
	}
	t.Fatal("el registry del doctor no incluye `swallowed_envelope`: el chequeo existe y nadie lo corre")
}
