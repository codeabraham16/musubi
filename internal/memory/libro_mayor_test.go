package memory

import "testing"

// libro_mayor_test.go cubre los prefijos de LIBRO MAYOR que declara el DESPLIEGUE
// (ConflictOptions.LedgerPrefixes): géneros de nota que se leen y se citan, pero que nadie puede
// pedir que se REEMPLACEN.
//
// EL CASO REAL, medido en el cerebro central el 2026-08-17: 465 relaciones pendientes, el 83% en la
// franja 0,30-0,35 pegada al piso del detector. No eran contradicciones: eran 27 notas
// `terminales/` —despachos entre agentes— pareándose entre sí por la PLANTILLA que comparten
// (cabeceras, emoji, nombres de destinatario). 27×26/2 = 351, el grueso de la cola.

// despacho es el texto de una carta entre agentes. La plantilla compartida ES la causa del
// parecido, así que el test la reproduce en vez de inventar dos textos cualquiera.
const despacho = "EMISARIO a PRINCIPAL, acuse y cierre del despacho anterior: verifique tu cambio, " +
	"acepto una refutacion tuya y queda un tercer camino que no es de ninguno de los dos sino mio"

var soloTerminales = ConflictOptions{LedgerPrefixes: []string{"terminales/"}}

// L1 — EL CASO QUE MOTIVÓ LA GUARDA, de punta a punta. Dos despachos con la misma plantilla
// generan una relación que nunca va a tener veredicto; con el prefijo declarado no nacen.
func TestL1DosDespachosNoSePidenVeredicto(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservationTyped("BASE", "terminales/emisario-a-principal", despacho, 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("OTRO", "terminales/auditor-a-planificador", despacho+" y con el mismo membrete", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}

	// CONTROL: SIN declarar el prefijo la relación DEBE nacer. Sin esto, el test de abajo podría
	// estar verde porque el detector no detecta nada, y no probaría absolutamente nada.
	rels, err := e.DetectRelations("OTRO", ConflictOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) == 0 {
		t.Fatal("el control no sirve: sin la guarda el detector tampoco propuso nada")
	}

	// LO QUE SE PRUEBA: con el prefijo declarado, ese mismo par no propone nada.
	if err := e.SaveObservationTyped("TERCERO", "terminales/skills-a-emisario", despacho+" y con el mismo membrete tambien", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	conGuarda, err := e.DetectRelations("TERCERO", soloTerminales)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range conGuarda {
		t.Errorf("FUGA L1: nació una relación entre despachos (%s -> %s)", r.SourceID, r.TargetID)
	}
}

// ⚠️ L2 — LA ASIMETRÍA, que es lo que separa una REGLA de un MARTILLO. Un despacho SÍ puede
// envejecer una nota: la carta trae mediciones, y ese es su valor. Lo que no se puede es tacharla
// A ELLA. Si esto alguna vez se pone verde al revés, la guarda dejó de ser una regla.
func TestL2UnDespachoSiPuedeEnvejecerUnaNota(t *testing.T) {
	e := newTestEngine(t)

	// La NOTA es el target, y vive en el mismo dominio para que el guardia de dominios no
	// interfiera: acá se prueba la asimetría del libro mayor, no aquélla.
	if err := e.SaveObservationTyped("NOTA", "terminales/estado-del-censo", despacho, 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("CARTA", "terminales/emisario-a-principal", despacho+" y con el mismo membrete", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}

	// Con el prefijo declarado AMBAS son libro mayor, así que para probar la asimetría hace falta
	// que sólo la SOURCE lo sea. Se declara un prefijo que matchea la carta y no la nota.
	opts := ConflictOptions{LedgerPrefixes: []string{"terminales/emisario-"}}
	rels, err := e.DetectRelations("CARTA", opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) == 0 {
		t.Fatal("un despacho como SOURCE debe poder relacionarse con una nota: la guarda mira sólo el TARGET")
	}
}

// L3 — SIN CONFIGURAR, NADA CAMBIA. El cero de LedgerPrefixes tiene que dejar el motor
// bit-idéntico: es la condición para que esto se pueda soltar sin riesgo en cualquier instalación.
func TestL3SinPrefijosElComportamientoNoCambia(t *testing.T) {
	casos := []struct {
		nombre string
		opts   ConflictOptions
	}{
		{"nil", ConflictOptions{}},
		{"vacío", ConflictOptions{LedgerPrefixes: []string{}}},
	}
	for _, c := range casos {
		if complementaryPair(obsRow{topicKey: "a/x"}, obsRow{topicKey: "terminales/y"}, c.opts) {
			t.Errorf("%s: sin prefijos declarados, `terminales/` no puede ser libro mayor", c.nombre)
		}
	}
}

// ⚠️ L4 — UN PREFIJO VACÍO NO APAGA LA DETECCIÓN ENTERA. `strings.HasPrefix(x, "")` es SIEMPRE
// true, así que un `ledger_prefixes: [""]` mal tipeado silenciaría la cola completa sin un solo
// error. Se ignora, que es fallar-ABIERTO: cuesta ruido, no memoria perdida.
func TestL4UnPrefijoVacioNoSilenciaTodo(t *testing.T) {
	opts := ConflictOptions{LedgerPrefixes: []string{""}}
	if complementaryPair(obsRow{topicKey: "a/x"}, obsRow{topicKey: "b/y"}, opts) {
		t.Error("un prefijo vacío apagó la detección entera: tiene que ignorarse")
	}
	// Y el prefijo válido que lo acompaña sigue valiendo: se ignora el vacío, no la lista.
	conAmbos := ConflictOptions{LedgerPrefixes: []string{"", "terminales/"}}
	if !complementaryPair(obsRow{topicKey: "a/x"}, obsRow{topicKey: "terminales/y"}, conAmbos) {
		t.Error("ignorar el prefijo vacío no puede invalidar a los demás de la lista")
	}
}

// ⚠️ L5 — NO SE EXIME DEL GUARDIA DE DOMINIOS, y ésta es la diferencia deliberada con
// `historicalRecord`, que sí está exento en las dos guardas.
//
// El porqué: un commit es evidencia sobre EL MUNDO —«feat: migrar de X a Y» envejece una nota de
// cualquier tema—, así que cruzar dominios es correcto para él. Un género declarado por
// configuración no reclama esa autoridad: sólo dice «esto no se tacha». Eximirlo AGREGARÍA pares
// cruzados, o sea lo contrario de para lo que se pidió la guarda.
//
// Este test es la red: si alguien cablea LedgerPrefixes dentro de dominiosAjenos, se pone rojo.
func TestL5ElLibroMayorDeclaradoNoCruzaDominios(t *testing.T) {
	if !dominiosAjenos(obsRow{topicKey: "terminales/emisario-a-principal"}, obsRow{topicKey: "roadmap/track-21"}) {
		t.Error("un prefijo declarado NO debe eximir del guardia de dominios: eso agregaría pares cruzados")
	}
}
