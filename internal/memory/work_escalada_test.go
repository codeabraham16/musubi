package memory

import (
	"strings"
	"testing"
	"time"
)

// work_escalada_test.go — «una escalada que no dice qué pasó no es una escalada».
//
// EL DEFECTO QUE CIERRA. El dead-letter escribía un string FIJO: «lease agotado: superó el máximo
// de reintentos». Con ese mensaje, dos situaciones opuestas son indistinguibles:
//
//	cinco agentes distintos muriendo en tiempos dispares  → la infraestructura está inestable
//	el MISMO agente muriendo cinco veces a los ~30 s      → la unidad tiene un cuelgue reproducible
//
// El segundo es un bug esperando a que alguien lo mire. El primero es reintentar y seguir. Que el
// sistema diera la misma respuesta a las dos preguntas es lo que se arregla acá.

func t0(seg int) time.Time {
	return time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC).Add(time.Duration(seg) * time.Second)
}

func logDe(pares ...interface{}) string {
	var b strings.Builder
	for i := 0; i < len(pares); i += 2 {
		b.WriteString(entradaDeReclamo(pares[i].(string), t0(pares[i+1].(int))))
	}
	return b.String()
}

// ★ E1 — EL MISMO AGENTE REPETIDO SE NOMBRA COMO PATRÓN, NO COMO LISTA.
func TestLaEscaladaDelataElCuelgueReproducible(t *testing.T) {
	msg := escaladaDesdeHistoria(logDe("a1", 0, "a1", 30, "a1", 60), 3)

	if !strings.Contains(msg, "3 reclamos") {
		t.Errorf("la escalada tiene que decir cuántos reclamos hubo; dijo: %q", msg)
	}
	if !strings.Contains(msg, "cuelgue reproducible") {
		t.Errorf("con el mismo agente tres veces, la escalada tiene que apuntar al cuelgue y no dejar la deducción al lector; dijo: %q", msg)
	}
	if strings.Contains(msg, "el patrón apunta al entorno") {
		t.Errorf("no puede culpar al entorno cuando siempre fue el mismo agente; dijo: %q", msg)
	}
	// Las retenciones son la evidencia de que el veredicto no es una corazonada.
	if !strings.Contains(msg, "Retuvo:") || !strings.Contains(msg, "30s") {
		t.Errorf("faltan las retenciones, que son lo que muestra el tiempo de muerte constante; dijo: %q", msg)
	}
}

// ★ E2 — AGENTES DISTINTOS APUNTAN AL ENTORNO. El veredicto opuesto, sobre el mismo mecanismo.
//
// Sin este caso, E1 pasaría con una implementación que dijera «cuelgue reproducible» SIEMPRE.
func TestLaEscaladaDistingueLaInfraestructura(t *testing.T) {
	msg := escaladaDesdeHistoria(logDe("a1", 0, "a2", 12, "a3", 90), 3)

	if !strings.Contains(msg, "3 agentes distintos") {
		t.Errorf("con tres agentes distintos tiene que decirlo; dijo: %q", msg)
	}
	if !strings.Contains(msg, "apunta al entorno") {
		t.Errorf("el veredicto para agentes distintos es el entorno, no la unidad; dijo: %q", msg)
	}
	if strings.Contains(msg, "cuelgue reproducible") {
		t.Errorf("no puede diagnosticar un cuelgue cuando cada reclamo fue de un agente distinto; dijo: %q", msg)
	}
}

// ★ E3 — SIN HISTORIA, EL MENSAJE DE SIEMPRE. No se inventa un diagnóstico.
//
// Una base recién migrada tiene claim_log vacío en las unidades que ya estaban en vuelo. Perder el
// detalle es aceptable; fabricarlo no.
func TestSinHistoriaLaEscaladaNoInventa(t *testing.T) {
	msg := escaladaDesdeHistoria("", 5)
	if !strings.Contains(msg, "superó el máximo de reintentos") {
		t.Errorf("sin historia tiene que caer al mensaje base; dijo: %q", msg)
	}
	if strings.Contains(msg, "reclamos:") || strings.Contains(msg, "Retuvo") {
		t.Errorf("sin historia no puede afirmar nada sobre los reclamos; dijo: %q", msg)
	}
	if !strings.Contains(msg, "(5)") {
		t.Errorf("el mensaje base tiene que decir CUÁL era el máximo; dijo: %q", msg)
	}
}

// ★ E4 — EL SEPARADOR NO SE PUEDE FALSIFICAR DESDE EL NOMBRE DEL AGENTE.
//
// El nombre del agente es texto de afuera y acá hace de separador. Un agente llamado "a\tb"
// partiría la línea y la historia diría cualquier cosa — o peor, un agente podría inyectar
// entradas falsas en el log de otro.
func TestElNombreDelAgenteNoRompeElLog(t *testing.T) {
	log := entradaDeReclamo("malo\tinyectado\notro\t2026-01-01T00:00:00Z", t0(0))
	rs := parseClaimLog(log)
	if len(rs) != 1 {
		t.Fatalf("un solo reclamo tiene que producir UNA entrada, produjo %d: %+v", len(rs), rs)
	}
	if strings.ContainsAny(rs[0].Agente, "\t\n") {
		t.Errorf("el agente quedó con separadores adentro: %q", rs[0].Agente)
	}
}

// ★ E5 — EL LOG A MEDIO ESCRIBIR NO PUEDE IMPEDIR UNA ESCALADA.
//
// El parseo alimenta un mensaje de diagnóstico. Si una línea corrupta lo hiciera fallar, el modo de
// falla sería perder la escalada entera — que es lo único que el humano iba a recibir.
func TestUnaLineaRotaNoTumbaLaHistoria(t *testing.T) {
	log := "basura sin tab\n" + entradaDeReclamo("a1", t0(0)) + "a2\tfecha-invalida\n" + entradaDeReclamo("a2", t0(45))
	rs := parseClaimLog(log)
	if len(rs) != 2 {
		t.Fatalf("las dos líneas buenas tienen que sobrevivir a las dos rotas, quedaron %d: %+v", len(rs), rs)
	}
	if msg := escaladaDesdeHistoria(log, 2); !strings.Contains(msg, "2 reclamos") {
		t.Errorf("la escalada tiene que armarse con lo legible; dijo: %q", msg)
	}
}

// ★ E6 — LA HISTORIA LLEGA AL RESULTADO DE LA UNIDAD, NO SÓLO A UNA FUNCIÓN PURA.
//
// E1-E5 prueban el formateador. Esta prueba recorre el camino REAL —claim, lease vencido, reclamo
// que dispara el dead-letter— y comprueba que el resultado persistido lleve el diagnóstico. Sin
// ella, todo lo anterior podría estar bien y el cable desconectado.
func TestElDeadLetterPersisteLaHistoria(t *testing.T) {
	e := newTestEngine(t)

	if _, err := e.CreateWorkBatch("b-esc", []WorkUnitSpec{{Title: "u1", Spec: "algo"}}); err != nil {
		t.Fatalf("CreateWorkBatch: %v", err)
	}

	// Cada vuelta: reclamar y envejecer el lease, que es como se ve un dueño que crasheó. Así la
	// siguiente encuentra una huérfana y vuelve a reclamarla, subiendo attempts.
	var id string
	for i := 0; i < 3; i++ {
		u, ok, err := e.ClaimWorkUnit("b-esc", "siempre-el-mismo", 60, 3)
		if err != nil || !ok {
			t.Fatalf("reclamo %d: ok=%v err=%v", i+1, ok, err)
		}
		id = u.ID
		ageLease(t, e, id)
	}
	// El cuarto reclamo encuentra attempts>=3 y la dead-lettea.
	if _, _, err := e.ClaimWorkUnit("b-esc", "otro", -1, 3); err != nil {
		t.Fatalf("reclamo que dispara el dead-letter: %v", err)
	}

	b, err := e.WorkBatchStatus("b-esc")
	if err != nil {
		t.Fatalf("WorkBatchStatus: %v", err)
	}
	if len(b.Units) != 1 {
		t.Fatalf("esperaba 1 unidad, hay %d", len(b.Units))
	}
	u := b.Units[0]
	if u.Status != WorkFailed {
		t.Fatalf("la unidad tenía que quedar failed, quedó %q", u.Status)
	}
	if !strings.Contains(u.Result, "siempre-el-mismo") {
		t.Errorf("el resultado no nombra al agente que la retuvo: %q", u.Result)
	}
	if !strings.Contains(u.Result, "cuelgue reproducible") {
		t.Errorf("el resultado no lleva el diagnóstico: %q", u.Result)
	}
}
