package receipt

import (
	"strings"
	"testing"
	"time"
)

// Banco de los hallazgos congelados. Cada test declara UN invariante, nombrado H<n>.

var t0 = time.Unix(0, 0).UTC()

func congelado(t *testing.T, id, cuerpo, tree string) map[string]Finding {
	t.Helper()
	f, err := Congelar(id, cuerpo, tree, t0)
	if err != nil {
		t.Fatalf("Congelar: %v", err)
	}
	return map[string]Finding{f.ID: f}
}

// --- H0 🔴: Compute NO se movió ---------------------------------------------------------

// TestH0ComputeSigueDandoLosMismosBytes es una línea base, no una prueba de comportamiento.
//
// De la salida de Compute dependen TODOS los recibos vigentes: si cambiara un solo byte, cada
// recibo aprobado dejaría de cubrir su árbol y el hook pre-push bloquearía los push de todo el
// mundo. Los hexes de acá se obtuvieron CORRIÉNDOLO antes de tocar nada — no se escriben de
// memoria, que es como se congela una línea base equivocada y se la cree.
func TestH0ComputeSigueDandoLosMismosBytes(t *testing.T) {
	casos := []struct {
		head, diff, unt, quiero string
	}{
		{"", "", "", "63cab8e921e413242a44bf4e8fdc999d3834c0883aed7afd6199c1ffa98c1948"},
		{"headA", "diffA", "untrackedA", "a60155e20334b17e31bd63a6090575cc37c08fdca8379fcb1a2b2121b4afa524"},
		{"9d7984e", "diff --git a/x b/x\n", "nuevo.go\n", "7d2ba058385d17e09d2057afeea42484947cdb23af3a5b7f76b80514864f3d02"},
	}
	for _, c := range casos {
		if got := Compute(c.head, c.diff, c.unt); got != c.quiero {
			t.Errorf("Compute(%q,%q,%q) cambió: obtuve %s, la línea base es %s.\n"+
				"Si esto se movió, TODOS los recibos vigentes se invalidaron y los push están bloqueados.",
				c.head, c.diff, c.unt, got, c.quiero)
		}
	}
}

// --- H1: canonizar CRLF, y SÓLO eso ------------------------------------------------------

func TestH1CanonizarNormalizaLosFinalesDeLinea(t *testing.T) {
	// La mitad que hace que el mecanismo sea usable: el mismo texto guardado por dos editores
	// distintos tiene que dar la misma huella, o en Windows cada verificación diría «mutó».
	if HashCuerpo("una linea\r\notra linea\r\n") != HashCuerpo("una linea\notra linea\n") {
		t.Error("CRLF y LF del mismo texto tienen que dar la misma huella: si no, el mecanismo se apaga solo en una semana")
	}
}

func TestH1CanonizarNoNormalizaNadaMas(t *testing.T) {
	// La otra mitad, y la peor si falla: normalizar de más hace que dos hallazgos con
	// significados DISTINTOS den la misma huella, y el congelado pasa a aprobar mutaciones reales.
	base := "el índice puede estar vacío"
	distintos := map[string]string{
		"la negación cambia el significado": "el índice no puede estar vacío",
		"mayúsculas":                        "El Índice Puede Estar Vacío",
		"espacio interno":                   "el  índice  puede  estar  vacío",
		"espacio al borde":                  "  el índice puede estar vacío  ",
		"puntuación":                        "el índice puede estar vacío.",
	}
	for nombre, otro := range distintos {
		if HashCuerpo(base) == HashCuerpo(otro) {
			t.Errorf("%s: dos textos distintos no pueden compartir huella, o el congelado aprueba mutaciones reales", nombre)
		}
	}
}

// --- H2: el cuerpo NO se guarda -----------------------------------------------------------

func TestH2ElCuerpoNoSeGuarda(t *testing.T) {
	secreto := "el token vive en Vaultwarden y el hallazgo lo nombra"
	cong := congelado(t, "d1/seguridad", secreto, "arbol1")

	raw, err := EncodeFindings(cong)
	if err != nil {
		t.Fatalf("EncodeFindings: %v", err)
	}
	// No guardar el cuerpo NO es ahorro de espacio: obliga a que quien verifica TENGA el texto.
	// Un verificador que puede leer el texto del propio registro no verifica, se mira al espejo.
	if strings.Contains(raw, secreto) {
		t.Errorf("el cuerpo no se guarda, sólo su huella; el registro serializado lo contiene:\n%s", raw)
	}
	if !strings.Contains(raw, cong["d1/seguridad"].BodyHash) {
		t.Error("la huella del cuerpo sí tiene que estar")
	}
}

// --- H3: los tres motivos de rechazo son distintos ----------------------------------------

func TestH3LosTresMotivosPidenAccionesDistintas(t *testing.T) {
	cuerpo := "el radio 0 no distingue «no arrastra a nadie» de «no puedo saberlo»"
	cong := congelado(t, "d1/correctitud", cuerpo, "arbol1")

	// (a) no está congelado
	d := Verificar(cong, "d1/inexistente", cuerpo, "arbol1")
	if d.Allowed || !strings.Contains(d.Reason, "congelalo") {
		t.Errorf("un id desconocido tiene que pedir CONGELAR; obtuve %+v", d)
	}
	// (b) el cuerpo mutó
	d = Verificar(cong, "d1/correctitud", cuerpo+" y algo más", "arbol1")
	if d.Allowed || !strings.Contains(d.Reason, "cuerpo") {
		t.Errorf("un cuerpo cambiado tiene que pedir RE-EMITIR; obtuve %+v", d)
	}
	// (c) el árbol se movió
	d = Verificar(cong, "d1/correctitud", cuerpo, "arbol2")
	if d.Allowed || !strings.Contains(d.Reason, "código cambió") {
		t.Errorf("un árbol distinto tiene que pedir RE-VERIFICAR; obtuve %+v", d)
	}
	// (d) todo en su lugar
	if d := Verificar(cong, "d1/correctitud", cuerpo, "arbol1"); !d.Allowed {
		t.Errorf("el mismo cuerpo sobre el mismo árbol tiene que pasar; obtuve %+v", d)
	}

	// Y los tres motivos tienen que ser DISTINTOS entre sí: colapsarlos haría que el agente
	// reintente la acción equivocada.
	razones := map[string]bool{
		Verificar(cong, "d1/inexistente", cuerpo, "arbol1").Reason:     true,
		Verificar(cong, "d1/correctitud", cuerpo+"x", "arbol1").Reason: true,
		Verificar(cong, "d1/correctitud", cuerpo, "arbol2").Reason:     true,
	}
	if len(razones) != 3 {
		t.Errorf("los tres rechazos tienen que dar tres mensajes distintos, obtuve %d", len(razones))
	}
}

// --- H4: la mutación silenciosa de PostPosture queda atrapada ------------------------------

func TestH4UnaPosturaReemplazadaEnSilencioNoPasaLaVerificacion(t *testing.T) {
	// El agujero real: PostPosture hace ON CONFLICT ... DO UPDATE SET stance=excluded.stance, o
	// sea que re-postear con la misma etiqueta reemplaza el texto anterior sin error y sin rastro.
	// El tally sería fiel a una discusión que ya no existe. Congelado, eso se ve.
	original := "REFUTO: el gate no cubre los archivos sin trackear"
	cong := congelado(t, "d7/seguridad", original, "arbol1")

	reemplazado := "APRUEBO: no encontré nada"
	if d := Verificar(cong, "d7/seguridad", reemplazado, "arbol1"); d.Allowed {
		t.Error("una postura reemplazada en silencio tiene que fallar la verificación: es exactamente el agujero que esto cierra")
	}
}

// --- H5: la poda por árbol -----------------------------------------------------------------

func TestH5LaPodaPorArbolDescartaLoViejo(t *testing.T) {
	a, _ := Congelar("d1/a", "hallazgo a", "arbol1", t0)
	b, _ := Congelar("d1/b", "hallazgo b", "arbol1", t0)
	c, _ := Congelar("d2/c", "hallazgo c", "arbol2", t0)
	cong := map[string]Finding{a.ID: a, b.ID: b, c.ID: c}

	vivos, podados := PodarPorArbol(cong, "arbol2")
	if podados != 2 || len(vivos) != 1 {
		t.Errorf("tras un fix, los hallazgos del árbol viejo se caen solos: quería 2 podados y 1 vivo, obtuve %d y %d", podados, len(vivos))
	}
	if _, ok := vivos["d2/c"]; !ok {
		t.Error("el que sobrevive es el del árbol de hoy")
	}
}

// --- H6: la colección aguanta N y se serializa determinista ---------------------------------

func TestH6LaColeccionGuardaVariosYEsDeterminista(t *testing.T) {
	cong := map[string]Finding{}
	for _, id := range []string{"d1/seguridad", "d1/correctitud", "d1/repro"} {
		f, err := Congelar(id, "cuerpo de "+id, "arbol1", t0)
		if err != nil {
			t.Fatalf("Congelar %s: %v", id, err)
		}
		cong[f.ID] = f
	}
	raw, err := EncodeFindings(cong)
	if err != nil {
		t.Fatalf("EncodeFindings: %v", err)
	}
	// Determinista: dos colecciones iguales dan el mismo texto, o cada guardado parece un cambio.
	raw2, _ := EncodeFindings(cong)
	if raw != raw2 {
		t.Error("la serialización tiene que ser determinista (ids ordenados)")
	}
	vuelta := DecodeFindings(raw)
	if len(vuelta) != 3 {
		t.Errorf("un panel emite N hallazgos y los N tienen que sobrevivir el viaje; obtuve %d", len(vuelta))
	}
	for id, f := range cong {
		if vuelta[id].BodyHash != f.BodyHash || vuelta[id].Tree != f.Tree {
			t.Errorf("%s no volvió igual: %+v vs %+v", id, vuelta[id], f)
		}
	}
}

// --- H7: falla cerrado ante un registro corrupto --------------------------------------------

func TestH7UnRegistroCorruptoEsColeccionVacia(t *testing.T) {
	for _, raw := range []string{"", "   ", "{no es json", "null"} {
		cong := DecodeFindings(raw)
		if len(cong) != 0 {
			t.Errorf("%q tiene que degradar a colección vacía, obtuve %d", raw, len(cong))
		}
		// Y vacío significa que TODO hallazgo necesita congelarse de nuevo: falla cerrado.
		if d := Verificar(cong, "d1/x", "cuerpo", "arbol1"); d.Allowed {
			t.Errorf("con el registro corrupto nada puede estar verificado; obtuve %+v", d)
		}
	}
}

// --- H8: congelar exige id y cuerpo ----------------------------------------------------------

func TestH8NoSeCongelaLoVacio(t *testing.T) {
	if _, err := Congelar("", "cuerpo", "arbol1", t0); err == nil {
		t.Error("sin id estable no hay forma de encontrar el hallazgo después")
	}
	if _, err := Congelar("d1/x", "   ", "arbol1", t0); err == nil {
		t.Error("un hallazgo vacío congelado no dejaría nada que verificar")
	}
}

// H1c: el salto FINAL no cambia el significado, el interno sí.
//
// Sale de un error real de mi propia verificación de punta a punta: congelé con `printf` (sin
// salto final) y verifiqué con `printf '%s\r\n'` (con salto), leí «mutó» y lo primero que pensé
// fue que la canonización estaba rota. No lo estaba: yo había cambiado el CONTENIDO. Pero el
// susto señaló un filo real —`echo` agrega salto, `printf` no, un archivo puede o no tenerlo— y
// ese filo apaga el mecanismo el primer día.
func TestH1ElSaltoFinalNoCuentaPeroElInternoSi(t *testing.T) {
	base := "REFUTO: el radio 0 no distingue dos cosas"
	iguales := []string{base, base + "\n", base + "\r\n", base + "\n\n\n"}
	for _, v := range iguales {
		if HashCuerpo(v) != HashCuerpo(base) {
			t.Errorf("%q tiene que dar la misma huella que el texto pelado: un salto al final no cambia el significado", v)
		}
	}
	// Pero el salto INTERNO sí es contenido: dos líneas no son una.
	if HashCuerpo("linea a\nlinea b") == HashCuerpo("linea a linea b") {
		t.Error("un salto interno separa dos líneas: no se puede colapsar sin cambiar el texto")
	}
	// Y el espacio interno sigue contando, como en H1b.
	if HashCuerpo("a  b") == HashCuerpo("a b") {
		t.Error("el espacio interno no se normaliza")
	}
}
