package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/receipt"
)

// Banco del gate de revisión. Cada test declara UN invariante, nombrado G<n>, y hay
// un sabotaje que lo ataca en sabotajes_reviewgate.md — el invariante que el test
// dice cubrir, no una validación cualquiera que lo tape por defensa en profundidad.

// fakeGateProbe sustituye a git con entradas fijas y CUENTA las consultas. El conteo
// no es decorativo: dos invariantes del gate (G3 y G8) son sobre lo que NO se llega a
// preguntar, y sin contador esos tests no podrían distinguir "no preguntó" de
// "preguntó y el resultado no cambió nada".
type fakeGateProbe struct {
	trabajo   trabajoSinRevisar
	fp        string
	errPend   error
	errHuella error

	nPend   int
	nHuella int
}

func (f *fakeGateProbe) pendiente() (trabajoSinRevisar, error) {
	f.nPend++
	return f.trabajo, f.errPend
}

func (f *fakeGateProbe) huella() (string, error) {
	f.nHuella++
	return f.fp, f.errHuella
}

// reciboAprobado deja en meta un recibo aprobado para la huella dada.
func reciboAprobado(t *testing.T, store *fakeTurnStore, fp string) {
	t.Helper()
	raw, err := receipt.Encode(receipt.Receipt{
		Fingerprint: fp,
		Head:        "deadbeef",
		Verdict:     receipt.Approved,
		Reason:      "revisado en el banco",
		IssuedBy:    "test",
		IssuedAt:    time.Unix(0, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("no pude codificar el recibo: %v", err)
	}
	if err := store.SetMeta(receipt.MetaKey, raw); err != nil {
		t.Fatalf("no pude guardar el recibo: %v", err)
	}
}

// sobreElUmbral es trabajo que cruza por archivos Y por líneas, para que un test que
// no habla del umbral no dependa de cuál de los dos cortes lo disparó.
func sobreElUmbral() trabajoSinRevisar {
	return trabajoSinRevisar{archivos: 3, lineas: 120}
}

// --- G1: hay trabajo sin revisar y nadie lo revisó => el gate avisa ---------------

func TestG1ReviewGateAvisaConTrabajoSinRevisar(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}

	out := buildReviewGate(store, "s1", probe)
	if out == "" {
		t.Fatal("con 3 archivos y 120 líneas sin revisar el gate tiene que avisar")
	}
	// El bloque tiene que decir QUÉ hacer, no sólo que algo pasa: un aviso sin el
	// nombre de la skill ni el comando obliga a adivinar, y quien lo lee es un modelo.
	for _, must := range []string{"adversarial-review", "musubi receipt emit", "3 archivos", "120 líneas"} {
		if !strings.Contains(out, must) {
			t.Errorf("el bloque del gate debe mencionar %q; obtuve:\n%s", must, out)
		}
	}
}

// --- G2: se avisa UNA vez por sesión, y el estado no sangra entre sesiones --------

func TestG2ReviewGateAvisaUnaSolaVezPorSesion(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}

	if buildReviewGate(store, "s1", probe) == "" {
		t.Fatal("el primer turno de la sesión tiene que avisar")
	}
	if seg := buildReviewGate(store, "s1", probe); seg != "" {
		t.Errorf("el segundo turno de la MISMA sesión tiene que callar; obtuve:\n%s", seg)
	}
}

func TestG2ReviewGateVuelveAAvisarEnUnaSesionNueva(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}

	buildReviewGate(store, "s1", probe)
	// Sin el session_id en la clave, la marca de "ya avisé" SANGRA: la sesión nueva
	// nace creyendo que ya avisó y el gate no vuelve a hablar nunca más.
	if out := buildReviewGate(store, "s2", probe); out == "" {
		t.Error("una sesión nueva con el mismo trabajo sin revisar tiene que volver a avisar")
	}
}

// --- G3: el interruptor apaga el gate ANTES de gastar en git ---------------------

func TestG3ReviewGateApagadoNoLlegaAConsultarGit(t *testing.T) {
	t.Setenv("MUSUBI_REVIEW_GATE", "0")
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}

	if out := buildReviewGate(store, "s1", probe); out != "" {
		t.Errorf("con MUSUBI_REVIEW_GATE=0 el gate tiene que callar; obtuve:\n%s", out)
	}
	// Callarse no alcanza: si igual consultara git, apagar el gate seguiría costando
	// dos subprocesos en CADA prompt. El interruptor tiene que salir gratis.
	if probe.nPend != 0 || probe.nHuella != 0 {
		t.Errorf("el gate apagado no debe tocar git; consultó pendiente=%d huella=%d", probe.nPend, probe.nHuella)
	}
}

// --- G4: un recibo que cubre ESTE árbol calla al gate ----------------------------

func TestG4ReviewGateCallaConReciboQueCubreElArbol(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}
	reciboAprobado(t, store, "huella-de-hoy")

	if out := buildReviewGate(store, "s1", probe); out != "" {
		t.Errorf("con un recibo aprobado para la huella actual el gate tiene que callar; obtuve:\n%s", out)
	}
}

// --- G5: un recibo de OTRA huella no cubre nada: el permiso venció ---------------

func TestG5ReviewGateAvisaSiElReciboEsDeOtraHuella(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}
	reciboAprobado(t, store, "huella-de-ayer")

	if buildReviewGate(store, "s1", probe) == "" {
		t.Error("un recibo emitido para otro estado del árbol no cubre el de ahora: el gate tiene que avisar")
	}
}

func TestG5ReviewGateAvisaSiElReciboEstaRechazado(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}
	raw, err := receipt.Encode(receipt.Receipt{Fingerprint: "huella-de-hoy", Verdict: receipt.Rejected, Reason: "no pasa"})
	if err != nil {
		t.Fatalf("no pude codificar el recibo: %v", err)
	}
	_ = store.SetMeta(receipt.MetaKey, raw)

	if buildReviewGate(store, "s1", probe) == "" {
		t.Error("un recibo RECHAZADO no es permiso: el gate tiene que avisar")
	}
}

// --- G6: la consulta cara sólo se paga si hay un recibo que podría cubrirla -------

func TestG6ReviewGateNoPideLaHuellaCuandoNoHayRecibo(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}

	buildReviewGate(store, "s1", probe)
	// La huella cuesta el diff COMPLETO. En el caso común —nadie emitió recibo
	// todavía— no hace falta para nada, y pedirla igual haría caro el camino frecuente.
	if probe.nHuella != 0 {
		t.Errorf("sin recibo en meta no hay nada que comparar: no debe pedirse la huella (se pidió %d veces)", probe.nHuella)
	}
	if probe.nPend != 1 {
		t.Errorf("el conteo de trabajo pendiente debe pedirse exactamente una vez, se pidió %d", probe.nPend)
	}
}

// --- G7: el gate falla ABIERTO ---------------------------------------------------

func TestG7ReviewGateCallaSiGitFalla(t *testing.T) {
	store := newFakeTurnStore()
	// El trabajo va CARGADO a propósito: si la sonda fallara y el conteo quedara en
	// cero, el gate callaría por el umbral y no por haber honrado el error — el test
	// pasaría sin medir nada. Cargado, la única razón posible del silencio es el error.
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), errPend: errors.New("fatal: not a git repository")}

	if out := buildReviewGate(store, "s1", probe); out != "" {
		t.Errorf("fuera de un repo git el gate tiene que callar, no romper el turno; obtuve:\n%s", out)
	}
}

func TestG7ReviewGateMudoSinSonda(t *testing.T) {
	store := newFakeTurnStore()
	if out := buildReviewGate(store, "s1", nil); out != "" {
		t.Errorf("sin sonda de git no hay nada que medir: el gate tiene que callar; obtuve:\n%s", out)
	}
}

func TestG7ReviewGateMudoSinMemoria(t *testing.T) {
	probe := &fakeGateProbe{trabajo: sobreElUmbral()}
	if out := buildReviewGate(nil, "s1", probe); out != "" {
		t.Errorf("sin memoria el gate tiene que callar; obtuve:\n%s", out)
	}
}

// --- G8: el umbral es un O, no un Y ----------------------------------------------

func TestG8CruzaUmbral(t *testing.T) {
	casos := []struct {
		nombre string
		t      trabajoSinRevisar
		quiero bool
	}{
		{"un archivo chico no alcanza", trabajoSinRevisar{archivos: 1, lineas: 5}, false},
		{"nada tocado", trabajoSinRevisar{}, false},
		{"dos archivos aunque sean chicos", trabajoSinRevisar{archivos: 2, lineas: 4}, true},
		{"un archivo con muchas líneas", trabajoSinRevisar{archivos: 1, lineas: 40}, true},
		{"justo debajo por los dos lados", trabajoSinRevisar{archivos: 1, lineas: 39}, false},
	}
	for _, c := range casos {
		if got := c.t.cruzaUmbral(); got != c.quiero {
			t.Errorf("%s: cruzaUmbral(%+v)=%v, quería %v", c.nombre, c.t, got, c.quiero)
		}
	}
}

// --- G9: sólo cuenta producción --------------------------------------------------

func TestG9ContarNumstatIgnoraTestsDocsEImagenes(t *testing.T) {
	salida := strings.Join([]string{
		"10\t2\tcmd/musubi/reviewgate.go",
		"300\t0\tcmd/musubi/reviewgate_test.go",
		"120\t4\tCHANGELOG.md",
		"5\t5\tdocs/imagen.png",
		"7\t1\tinternal/receipt/receipt.go",
	}, "\n")

	got := contarNumstat(salida)
	// 2 archivos de producción (los .go que no son _test) y 10+2+7+1 líneas.
	if got.archivos != 2 {
		t.Errorf("archivos de producción: quería 2, obtuve %d", got.archivos)
	}
	if got.lineas != 20 {
		t.Errorf("líneas de producción: quería 20, obtuve %d", got.lineas)
	}
}

func TestG9ContarNumstatCuentaBinarioSinLineas(t *testing.T) {
	got := contarNumstat("-\t-\tassets/fuente.ttf\n3\t1\tmain.go")
	if got.archivos != 2 {
		t.Errorf("un binario es un archivo tocado: quería 2, obtuve %d", got.archivos)
	}
	if got.lineas != 4 {
		t.Errorf("un binario no aporta líneas: quería 4, obtuve %d", got.lineas)
	}
}

func TestG9ContarNumstatEntiendeLasDosFormasDeRenombre(t *testing.T) {
	// La forma CON LLAVES es la que muerde: "docs/{notas.md => otras.md}" tiene
	// extensión ".md}" —con la llave pegada— y no coincide con ninguna exclusión, así
	// que un doc renombrado contaría como producción. Ídem "_test.go", que deja de ser
	// sufijo. La forma simple, en cambio, no necesita el corte: el nombre nuevo ya va
	// último, así que la última extensión es la suya. Medido, no supuesto.
	if got := contarNumstat("0\t0\tdocs/{notas.md => otras.md}"); got.archivos != 0 {
		t.Errorf("un doc renombrado sigue siendo doc: quería 0 archivos, obtuve %d", got.archivos)
	}
	if got := contarNumstat("0\t0\tcmd/{a_test.go => b_test.go}"); got.archivos != 0 {
		t.Errorf("un test renombrado sigue siendo test: quería 0 archivos, obtuve %d", got.archivos)
	}
	// Y la forma simple tiene que seguir andando: acá el archivo PASÓ a ser producción.
	if got := contarNumstat("0\t0\tdocs/notas.md => internal/motor.go"); got.archivos != 1 {
		t.Errorf("un doc que pasó a .go ahora es producción: quería 1 archivo, obtuve %d", got.archivos)
	}
}

func TestG9ContarUntrackedCuentaArchivosYLineas(t *testing.T) {
	dir := t.TempDir()
	escribir := func(nombre, contenido string) {
		ruta := filepath.Join(dir, nombre)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escribir("nuevo.go", strings.Repeat("linea\n", 50))
	escribir("nuevo_test.go", strings.Repeat("linea\n", 900))
	escribir("LEEME.md", strings.Repeat("linea\n", 900))

	got := contarUntracked(dir, "nuevo.go\nnuevo_test.go\nLEEME.md\n")
	if got.archivos != 1 {
		t.Errorf("sólo nuevo.go es producción: quería 1 archivo, obtuve %d", got.archivos)
	}
	// Un archivo sin trackear es trabajo 100 %% sin revisar: sus líneas cuentan enteras.
	// Si no se contaran, un solo archivo nuevo de 500 líneas no dispararía el gate.
	if got.lineas != 50 {
		t.Errorf("las líneas del archivo nuevo tienen que contar: quería 50, obtuve %d", got.lineas)
	}
}

// --- G10: el gate viaja dentro del envelope del hook, y sólo con sonda -----------

func TestG10TurnOutputConSondaTraeElGate(t *testing.T) {
	store := newFakeTurnStore()
	probe := &fakeGateProbe{trabajo: sobreElUmbral(), fp: "huella-de-hoy"}
	in := `{"prompt":"seguimos","session_id":"g1"}`

	out := turnOutputWith(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, probe, strings.NewReader(in), nil)
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "[Musubi — revisión]") {
		t.Errorf("el gate tiene que llegar dentro del additionalContext; obtuve:\n%s", ctx)
	}
	// Y su costo tiene que quedar imputado: una superficie inyectada que no se
	// contabiliza hace que el ledger sub-reporte el gasto real de Musubi.
	if store.ledger["review_gate"] <= 0 {
		t.Errorf("el bloque del gate debe imputarse al ledger en la superficie review_gate; ledger=%v", store.ledger)
	}
}

func TestG10TurnOutputSinSondaNoTraeElGate(t *testing.T) {
	store := newFakeTurnStore()
	in := `{"prompt":"seguimos","session_id":"g1"}`

	out := turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	_, ctx := hookAdditionalContext(t, out)
	if strings.Contains(ctx, "[Musubi — revisión]") {
		t.Errorf("sin sonda el gate no debe aparecer (es lo que mantiene a los demás tests independientes del árbol real); obtuve:\n%s", ctx)
	}
}

// G11: el gate no avisa sobre su PROPIO workspace.
//
// Encontrado en una prueba de punta a punta: en un repo recién creado, `.musubi/config.yaml` y
// `.musubi/config.example.yaml` quedan sin trackear y contaban como dos archivos de producción —
// justo el umbral. O sea que el gate se avisaba a sí mismo. Un aviso que salta por el ruido de la
// propia herramienta es el que enseña a ignorar la herramienta.
func TestG11ElGateNoCuentaSuPropioWorkspace(t *testing.T) {
	for _, ruta := range []string{".musubi/config.yaml", ".musubi/config.example.yaml", "sub/.musubi/config.yaml"} {
		if esProduccion(ruta) {
			t.Errorf("%q es estado de Musubi, no código de producción de nadie", ruta)
		}
	}
	// Pero un archivo que sólo MENCIONA musubi sigue contando: la exclusión es del directorio.
	if !esProduccion("internal/musubi/motor.go") {
		t.Error("la exclusión es del directorio .musubi/, no de todo lo que se llame musubi")
	}
}
