package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"musubi/internal/cognition"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/skills"
)

// Invariantes del DECIDIDOR DE SKILLS (spec skills-decididor, R1–R12). Las pruebas llaman a
// s.exigirSkillSiCorresponde directo con un RecallResult, un reflejo y un arsenal armados a mano
// —igual que las del juez llaman a rerankIfEnabled—: deterministas y liberadas del ranking y del
// disco, que no es lo que miden. Cada prueba nombra el SABOTAJE que la pone en rojo, y ese sabotaje
// ataca el invariante que la prueba declara (no una validación lateral).

// sentinelaCuerpo es texto que SÓLO existe en el `rules` (el cuerpo) de una skill del arsenal. Si
// aparece en el prompt del motor o en la línea de exigencia, el cuerpo se filtró (R7).
const sentinelaCuerpo = "SENTINELA_CUERPO_NO_DEBE_VIAJAR"

// respuestaElige / respuestaNinguna son las dos formas de respuesta del motor que ElegirSkill
// parsea: un objeto JSON con la skill elegida, o con skill vacía (ninguna aplica).
const (
	respuestaElige   = `{"skill":"go-hygiene","razon":"la tarea toca archivos .go"}`
	respuestaNinguna = `{"skill":"","razon":"ninguna aplica con claridad"}`
)

// arsenalDecididor es un catálogo con DOS skills, una de ellas ("go-hygiene") con un cuerpo
// distintivo. El cuerpo existe a propósito: sin él, la aserción de R7 «el cuerpo no viaja» sería
// vacua (no habría nada que pudiera filtrarse).
func arsenalDecididor() []skills.Skill {
	return []skills.Skill{
		{Name: "go-hygiene", Description: "higiene de Go: errores con %w, locks, nada de panic",
			Rules: sentinelaCuerpo, AlwaysBecause: "cuando tocás archivos .go"},
		{Name: "sdd-flow", Description: "flujo SDD guiado por fases",
			Rules: "otro cuerpo cualquiera", AlwaysBecause: "al implementar un cambio no trivial"},
	}
}

// ctxConProyecto arma un contexto con un principal writer que DECLARA un project_id: el pid es la
// cohorte del A/B (BrazoAB), así que las pruebas que dependen del brazo necesitan controlarlo.
func ctxConProyecto(nombre, pid string) context.Context {
	return withPrincipal(context.Background(), &Principal{Name: nombre, Role: RoleWriter, ProjectID: pid})
}

// pidDeBrazo encuentra un project_id que HOY caiga en el brazo pedido. BrazoAB cruza por día, así
// que un pid fijo volaría de brazo entre días; buscar uno para la fecha de la corrida mantiene la
// prueba determinista sin importar el día. ~la mitad de los candidatos caen en cada brazo.
func pidDeBrazo(t *testing.T, arm string) string {
	t.Helper()
	ahora := time.Now()
	for i := 0; i < 1000; i++ {
		pid := fmt.Sprintf("proj-decididor-%d", i)
		if memory.BrazoAB(pid, ahora) == arm {
			return pid
		}
	}
	t.Fatalf("no encontré un project_id que caiga en el brazo %q hoy", arm)
	return ""
}

// servidorDecididor arma un servidor con motor, el DIAL DEL DECIDIDOR encendido y el presupuesto en
// maxPorHora. Engine propio (sin sembrar: el decididor no recupera observaciones) para poder leer la
// tabla del A/B. metrics listo para contar las degradaciones (R5).
func servidorDecididor(t *testing.T, motor cognition.Provider, maxPorHora int) (*McpServer, *memory.DbEngine) {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	si := true
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake", DecididorSkills: &si}),
		WithMotorQuota(maxPorHora))
	s.metrics = &serverMetrics{}
	return s, engine
}

// filaDecididorAB devuelve la fila (pid, arm) del ledger del A/B, o la fila cero si no existe.
func filaDecididorAB(t *testing.T, e *memory.DbEngine, pid, arm string) memory.DecididorABRow {
	t.Helper()
	rows, err := e.DecididorAB(context.Background())
	if err != nil {
		t.Fatalf("DecididorAB: %v", err)
	}
	for _, r := range rows {
		if r.ProjectID == pid && r.Arm == arm {
			return r
		}
	}
	return memory.DecididorABRow{}
}

// contarFilasDecididor cuenta TODAS las filas del ledger del A/B: lo usan los guards (R1/R2/R3) para
// afirmar «ni siquiera se contó una ocasión».
func contarFilasDecididor(t *testing.T, e *memory.DbEngine) int {
	t.Helper()
	rows, err := e.DecididorAB(context.Background())
	if err != nil {
		t.Fatalf("DecididorAB: %v", err)
	}
	return len(rows)
}

// G1/R1 — LOCAL-FIRST: SIN MOTOR REAL, NO-OP TOTAL. Con NoopProvider (el daemon local) aunque el
// dial esté encendido, el decididor no hace NADA: ni llama, ni gatea, ni cuenta una ocasión del A/B.
// Se afirma sobre el ledger (0 filas) y no sobre la exigencia, porque Noop jamás produce una: la
// exigencia nula sería verde aun con el guard roto; la fila de A/B es lo que delata que corrió.
// Sabotaje: en decididorHabilitado, quitar `cognition.Enabled(s.cognition)` ⇒ con Noop sigue de
// largo, cuenta una ocasión ⇒ 1 fila ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="cognition.Enabled(s.cognition) &&"
// arnes: a="true &&"
func TestG1SinMotorRealEsNoOpTotal(t *testing.T) {
	s, e := servidorDecididor(t, cognition.NoopProvider{}, 100)
	ctx := ctxConProyecto("p1", "proj-g1")

	res := s.exigirSkillSiCorresponde(ctx, "g1 implementar el freno del motor en go",
		memory.RecallResult{}, nil, arsenalDecididor())

	if res.ExigenciaSkill != nil {
		t.Errorf("sin motor real no puede haber exigencia, salió %+v", res.ExigenciaSkill)
	}
	if n := contarFilasDecididor(t, e); n != 0 {
		t.Errorf("sin motor real el decididor no puede contar una ocasión del A/B; hay %d filas", n)
	}
}

// G2/R2 — DEFAULT-OFF: CON EL DIAL APAGADO, NO-OP TOTAL. Es el otro medio del guard y el estado por
// defecto del A/B (el puntero ausente ⇒ off). Con un motor REAL presente pero el dial en off, no se
// llama ni se cuenta. Apagar el dial ES el rollback (D6).
// Sabotaje: en decididorHabilitado, quitar `s.cognitionCfg.DecididorSkillsOn()` ⇒ con el dial off
// sigue de largo ⇒ 1 fila y/o 1 llamada ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="s.cognitionCfg.DecididorSkillsOn()"
// arnes: a="true"
func TestG2ConElDialApagadoEsNoOp(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	// SIN DecididorSkills: el dial apagado, que es el default.
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake"}),
		WithMotorQuota(100))
	ctx := ctxConProyecto("p1", "proj-g2")

	res := s.exigirSkillSiCorresponde(ctx, "g2 implementar el freno del motor en go",
		memory.RecallResult{}, nil, arsenalDecididor())

	if res.ExigenciaSkill != nil {
		t.Errorf("con el dial apagado no puede haber exigencia, salió %+v", res.ExigenciaSkill)
	}
	if n := motor.llamadas.Load(); n != 0 {
		t.Errorf("con el dial apagado el motor no puede ser llamado, hubo %d", n)
	}
	if n := contarFilasDecididor(t, engine); n != 0 {
		t.Errorf("con el dial apagado no puede contarse una ocasión del A/B; hay %d filas", n)
	}
}

// G3/R3 — GATE CONSERVADOR: SI EL REFLEJO YA PINÓ FUERTE, 0 LLAMADAS. Cuando el reflejo model-free
// ya acertó por glob (o alcance), el motor no agrega nada y no se gasta la llamada NI se cuenta la
// ocasión (el tratamiento ni se consideró). La query es sustantiva y el arsenal no vacío: lo ÚNICO
// que frena al motor es el match fuerte, para que el sabotaje no quede tapado por otro corte del gate.
// Sabotaje: en hayMatchFuerte devolver siempre false ⇒ el gate deja pasar ⇒ 1 llamada ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="if r.Matcheo == skills.PorGlob || r.Matcheo == skills.PorAlcance {"
// arnes: a="if false {"
func TestG3GateConMatchFuerteNoLlamaAlMotor(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	s, e := servidorDecididor(t, motor, 100)
	ctx := ctxConProyecto("p1", "proj-g3")
	reflejas := []skills.SkillResuelta{{Skill: skills.Skill{Name: "go-hygiene"}, Matcheo: skills.PorGlob}}

	res := s.exigirSkillSiCorresponde(ctx, "g3 implementar el freno del motor en go",
		memory.RecallResult{}, reflejas, arsenalDecididor())

	if n := motor.llamadas.Load(); n != 0 {
		t.Errorf("con un match fuerte del reflejo el motor no debe llamarse, hubo %d", n)
	}
	if res.ExigenciaSkill != nil {
		t.Errorf("con el gate cortando no puede montarse exigencia, salió %+v", res.ExigenciaSkill)
	}
	if n := contarFilasDecididor(t, e); n != 0 {
		t.Errorf("el gate corta ANTES de contar la ocasión; hay %d filas", n)
	}
}

// G4/R4 — TAREA SUSTANTIVA EN EL BRAZO ON: UNA LLAMADA Y LA EXIGENCIA MONTADA. El camino feliz del
// tratamiento. Gasta EXACTAMENTE una llamada, monta la exigencia estructurada con la skill elegida,
// y la ocasión queda contada (oportunidades=1, exigencias=1). La línea renderizada es UNA sola
// (D-exigencia) e imperativa.
// Sabotaje: en exigirSkillSiCorresponde, no setear res.ExigenciaSkill (dejar `exigio` en false) ⇒
// la exigencia queda nil ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de=`exigio := eleccion.Skill != ""`
// arnes: a="exigio := false"
func TestG4BrazoOnMontaExigenciaConUnaLlamada(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	s, e := servidorDecididor(t, motor, 100)
	pid := pidDeBrazo(t, memory.BrazoOn)
	ctx := ctxConProyecto("p1", pid)

	res := s.exigirSkillSiCorresponde(ctx, "g4 implementar el freno del motor en go",
		memory.RecallResult{}, nil, arsenalDecididor())

	if n := motor.llamadas.Load(); n != 1 {
		t.Fatalf("el brazo on gasta exactamente una llamada, hubo %d", n)
	}
	if res.ExigenciaSkill == nil {
		t.Fatal("el brazo on con una elección real tiene que montar la exigencia")
	}
	if res.ExigenciaSkill.Skill != "go-hygiene" {
		t.Errorf("la exigencia tiene que ser la skill elegida por el motor, salió %q", res.ExigenciaSkill.Skill)
	}
	linea := renderExigencia(*res.ExigenciaSkill)
	if !strings.Contains(linea, "cargá la skill") || !strings.Contains(linea, "go-hygiene") {
		t.Errorf("la línea tiene que ser imperativa y nombrar la skill, salió %q", linea)
	}
	if strings.Contains(linea, "\n") {
		t.Errorf("la exigencia es UNA línea (D-exigencia), salió con salto: %q", linea)
	}
	if fila := filaDecididorAB(t, e, pid, memory.BrazoOn); fila.Oportunidades != 1 || fila.Exigencias != 1 {
		t.Errorf("esperaba oport=1 exig=1 en el brazo on, obtuve %+v", fila)
	}
}

// G5/R5 — SIN PRESUPUESTO EL DECIDIDOR DEGRADA, NO EXIGE. Agotada la cuota, quien llamó pidió
// memoria: se devuelve el recall intacto, se cuenta la degradación (motorDenied) y la ocasión IGUAL
// cuenta (denominador honesto) pero sin exigencia. El presupuesto es 1: la 1ª llamada lo gasta, la
// 2ª (query distinta ⇒ miss) se topa con el freno.
// Sabotaje: quitar el bloque `if !s.hayPresupuestoDeMotor(ctx) { ... return res }` ⇒ la 2ª llama al
// motor igual y monta exigencia ⇒ la aserción «2ª sin exigencia» cae ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="logx.Warn(\"decididor de skills: sin presupuesto de motor, recall sin exigencia\",\n\t\t\t\t\"max_por_hora\", s.motorQuota.max)\n\t\t\ts.registrarOportunidadDecididor(ctx, pid, arm, false)\n\t\t\treturn res"
// arnes: a="logx.Warn(\"decididor de skills: sin presupuesto de motor, recall sin exigencia\",\n\t\t\t\t\"max_por_hora\", s.motorQuota.max)\n\t\t\ts.registrarOportunidadDecididor(ctx, pid, arm, false)"
func TestG5SinPresupuestoDegrada(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	s, e := servidorDecididor(t, motor, 1)
	pid := pidDeBrazo(t, memory.BrazoOn)
	ctx := ctxConProyecto("p1", pid)

	primera := s.exigirSkillSiCorresponde(ctx, "primera tarea del freno en Go",
		memory.RecallResult{}, nil, arsenalDecididor())
	if primera.ExigenciaSkill == nil {
		t.Fatal("la 1ra llamada está dentro del presupuesto y tiene que exigir")
	}

	segunda := s.exigirSkillSiCorresponde(ctx, "segunda tarea del freno en Go",
		memory.RecallResult{}, nil, arsenalDecididor())
	if segunda.ExigenciaSkill != nil {
		t.Errorf("sin presupuesto el decididor degrada, no exige; salió %+v", segunda.ExigenciaSkill)
	}
	if n := s.metrics.motorDenied.Load(); n != 1 {
		t.Errorf("motorDenied=%d, esperaba 1: una degradación silenciosa no se puede distinguir de «no aportó»", n)
	}
	if n := motor.llamadas.Load(); n != 1 {
		t.Errorf("la 2da fue rechazada por el freno y no puede haber llamado al motor; hubo %d llamadas", n)
	}
	// Las DOS ocasiones cuentan (denominador honesto); sólo la 1ª exigió.
	if fila := filaDecididorAB(t, e, pid, memory.BrazoOn); fila.Oportunidades != 2 || fila.Exigencias != 1 {
		t.Errorf("esperaba oport=2 exig=1, obtuve %+v", fila)
	}
}

// G6/R6 — BEST-EFFORT: MOTOR CAÍDO, IMPARSEABLE O VACÍO ⇒ RECALL INTACTO Y OCASIÓN CONTADA. Un fallo
// del motor nunca rompe ni altera el recall, y la ocasión igual cuenta (exigio=false) para no inflar
// la tasa de exigencia del brazo on con las veces que el motor no contestó.
// Sabotaje: en la rama `if err != nil` de exigirSkillSiCorresponde, quitar
// `s.registrarOportunidadDecididor(ctx, pid, arm, false)` ⇒ la ocasión del fallo no se cuenta ⇒ la
// aserción oport=1 cae ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="logx.Error(\"decididor de skills: recall sin exigencia (model-free intacto)\", \"error\", err)\n\t\t\ts.registrarOportunidadDecididor(ctx, pid, arm, false)"
// arnes: a="logx.Error(\"decididor de skills: recall sin exigencia (model-free intacto)\", \"error\", err)"
func TestG6MotorCaidoDegradaBestEffort(t *testing.T) {
	casos := map[string]*motorEspia{
		"motor caído":           {err: errors.New("connection refused")},
		"respuesta imparseable": {respuesta: "no sé, perdón"},
		"respuesta vacía":       {respuesta: ""},
	}
	for nombre, motor := range casos {
		t.Run(nombre, func(t *testing.T) {
			s, e := servidorDecididor(t, motor, 100)
			pid := pidDeBrazo(t, memory.BrazoOn)
			ctx := ctxConProyecto("p1", pid)
			original := memory.RecallResult{Items: []memory.RecallItem{{ID: "a", Gist: "una memoria"}}}

			res := s.exigirSkillSiCorresponde(ctx, "tarea del freno en Go con "+nombre, original, nil, arsenalDecididor())

			if res.ExigenciaSkill != nil {
				t.Errorf("con el motor fallando no puede montarse exigencia, salió %+v", res.ExigenciaSkill)
			}
			if len(res.Items) != 1 || res.Items[0].ID != "a" {
				t.Errorf("el recall model-free tiene que sobrevivir intacto, quedó %+v", res.Items)
			}
			if fila := filaDecididorAB(t, e, pid, memory.BrazoOn); fila.Oportunidades != 1 || fila.Exigencias != 0 {
				t.Errorf("la ocasión del fallo igual cuenta sin exigencia: esperaba oport=1 exig=0, obtuve %+v", fila)
			}
		})
	}
}

// G7/R7 — LA EXIGENCIA VIAJA SIN EL CUERPO, Y EL CUERPO TAMPOCO VIAJA AL MOTOR. Es el acople R7↔R10:
// si el cuerpo (las rules) viajara con la exigencia, el agente no necesitaría CARGAR la skill y el
// eje `loaded` —el numerador del A/B— dejaría de registrar, midiendo cero por construcción. La skill
// elegida TIENE cuerpo (sentinelaCuerpo) a propósito: sin cuerpo la prueba sería vacua.
// Sabotaje: en arsenalN1, incluir sk.Rules en la proyección (p.ej. Cuando: sk.AlwaysBecause+sk.Rules)
// ⇒ el cuerpo entra al prompt del motor ⇒ la sentinela aparece ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="Cuando:      sk.AlwaysBecause,"
// arnes: a="Cuando:      sk.AlwaysBecause + sk.Rules,"
func TestG7LaExigenciaNoLlevaElCuerpo(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	s, _ := servidorDecididor(t, motor, 100)
	pid := pidDeBrazo(t, memory.BrazoOn)
	ctx := ctxConProyecto("p1", pid)

	res := s.exigirSkillSiCorresponde(ctx, "g7 implementar el freno del motor en go",
		memory.RecallResult{}, nil, arsenalDecididor())

	if res.ExigenciaSkill == nil {
		t.Fatal("la prueba necesita una exigencia montada para mirar su contenido")
	}
	linea := renderExigencia(*res.ExigenciaSkill)
	if strings.Contains(linea, sentinelaCuerpo) {
		t.Errorf("el cuerpo de la skill se filtró en la línea de exigencia: %q", linea)
	}
	// Y el cuerpo ni siquiera viajó al motor: arsenalN1 lo dropea antes del prompt.
	system, user := motor.prompt()
	if strings.Contains(system, sentinelaCuerpo) || strings.Contains(user, sentinelaCuerpo) {
		t.Errorf("el cuerpo de la skill viajó al prompt del motor:\nsystem=%q\nuser=%q", system, user)
	}
}

// G8/R11 — UN ACIERTO DE CACHÉ NO LLAMA AL MOTOR NI CONSUME PRESUPUESTO. El freno vive DENTRO del
// miss: la misma tarea sobre el mismo catálogo se resuelve del caché, sin gastar una llamada ni
// cuota, aun con el presupuesto en 1 ya agotado por la 1ª. Query ÚNICA de esta prueba (decididorCache
// es global de paquete).
// Sabotaje: en decididorCachePut, NO guardar la elección (borrar `decididorCache[key] = e`) ⇒ la 2ª
// tarea idéntica vuelve a ser miss y el freno (presupuesto agotado en 1) la degrada en vez de servir
// del caché ⇒ la aserción «2ª con exigencia» cae ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="decididorCache[key] = e"
// arnes: a=""
func TestG8AciertoDeCacheNoGastaNiLlama(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	s, _ := servidorDecididor(t, motor, 1)
	pid := pidDeBrazo(t, memory.BrazoOn)
	ctx := ctxConProyecto("p1", pid)
	const query = "tarea unica de la prueba g8 cache del decididor en go"

	primera := s.exigirSkillSiCorresponde(ctx, query, memory.RecallResult{}, nil, arsenalDecididor())
	if primera.ExigenciaSkill == nil {
		t.Fatal("la 1ra (miss) tenía que llamar al motor y exigir")
	}
	segunda := s.exigirSkillSiCorresponde(ctx, query, memory.RecallResult{}, nil, arsenalDecididor())
	if segunda.ExigenciaSkill == nil {
		t.Error("el acierto de caché tiene que exigir igual; si degradó, el freno le cobró a algo que no llamó al motor")
	}
	if n := motor.llamadas.Load(); n != 1 {
		t.Fatalf("la 2da tenía que salir del caché: esperaba 1 llamada al motor, hubo %d", n)
	}
}

// G9/R12 — musubi_recall SIGUE READ-ONLY (ESCRITURA INCIDENTAL), NO SE PROMOVIÓ A MUTADOR. El A/B del
// decididor escribe bajo el RLock que la propia tool toma (withReadLock), de la MISMA clase incidental
// que el bump de access_count: un upsert atómico que SQLite serializa, no un read-modify-write de Go.
// Por eso la tool conserva su clasificación —ro=roLeeConEscrituraIncidental y lock=lockSelf—, que es
// lo que el despacho y el readOnlyHint de tools/list consumen (el barrido del hint vive en
// readonly_hint_test.go; el de aislamiento, en read_surface_class_test.go).
// Sabotaje: en registry.go, cambiar la clasificación de musubi_recall (ro ⇒ otra, o lock ⇒ lockRead)
// ⇒ esta prueba cae nombrando musubi_recall.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="// sin atender a nadie hasta 120 s. El handler acota su propia sección crítica.\n\t\t\tlock: lockSelf,"
// arnes: a="// sin atender a nadie hasta 120 s. El handler acota su propia sección crítica.\n\t\t\tlock: lockRead,"
func TestG9RecallSigueReadOnlyIncidental(t *testing.T) {
	s := NewMcpServer(nil, "", nil)
	var hallada bool
	for i := range s.tools {
		if s.tools[i].Name != "musubi_recall" {
			continue
		}
		hallada = true
		if s.tools[i].readOnly {
			t.Error("musubi_recall no es readOnly puro: tiene una escritura incidental (access_count, y ahora el A/B)")
		}
		if s.tools[i].ro != roLeeConEscrituraIncidental {
			t.Errorf("musubi_recall tiene que seguir clasificada roLeeConEscrituraIncidental; el A/B del decididor NO la promueve a mutador")
		}
		if s.tools[i].lock != lockSelf {
			t.Errorf("musubi_recall despacha con lockSelf (acota su propia sección crítica): el A/B escribe bajo ESE RLock, no bajo un candado del despacho")
		}
	}
	if !hallada {
		t.Fatal("musubi_recall no está en el registro")
	}
}

// Control de G1/G2/G3: con el decididor ENCENDIDO, motor real, en el brazo on y sin match fuerte,
// el decididor SÍ actúa. Sin este control, romper el decididor entero (que nunca exija nada) dejaría
// a G1/G2/G3 en verde —se satisfacen con un decididor que no hace nada— y la suite mentiría.
func TestG0ControlDecididorEncendidoActua(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	s, _ := servidorDecididor(t, motor, 100)
	ctx := ctxConProyecto("p1", pidDeBrazo(t, memory.BrazoOn))

	res := s.exigirSkillSiCorresponde(ctx, "g0 control del decididor encendido en go",
		memory.RecallResult{}, nil, arsenalDecididor())

	if res.ExigenciaSkill == nil || res.ExigenciaSkill.Skill != "go-hygiene" {
		t.Fatalf("el decididor encendido tenía que exigir go-hygiene, salió %+v", res.ExigenciaSkill)
	}
	if n := motor.llamadas.Load(); n != 1 {
		t.Errorf("tenía que llamar al motor una vez, hubo %d", n)
	}
}

// Control de R8: una elección "ninguna" (skill vacía) es una ocasión SIN exigencia, no un error. Es
// el caso más frecuente en producción (la mayoría de las tareas no exige una skill).
func TestNingunaEsOcasionSinExigencia(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaNinguna}
	s, e := servidorDecididor(t, motor, 100)
	pid := pidDeBrazo(t, memory.BrazoOn)
	ctx := ctxConProyecto("p1", pid)

	res := s.exigirSkillSiCorresponde(ctx, "una tarea que no exige ninguna skill puntual",
		memory.RecallResult{}, nil, arsenalDecididor())

	if res.ExigenciaSkill != nil {
		t.Errorf("«ninguna» no monta exigencia, salió %+v", res.ExigenciaSkill)
	}
	if fila := filaDecididorAB(t, e, pid, memory.BrazoOn); fila.Oportunidades != 1 || fila.Exigencias != 0 {
		t.Errorf("«ninguna» cuenta la ocasión sin exigencia: esperaba oport=1 exig=0, obtuve %+v", fila)
	}
}

// Control del brazo OFF: el control del A/B tiene la ocasión (cuenta en el denominador) pero NO
// recibe tratamiento —no se llama al motor ni se monta exigencia—.
// Sabotaje: borrar la rama `if arm == memory.BrazoOff { ... return res }` ⇒ el off llama al motor ⇒
// la aserción «0 llamadas» cae ⇒ rojo.
// arnes: archivo="internal/mcp/methods_cognition.go"
// arnes: de="if arm == memory.BrazoOff {"
// arnes: a="if false {"
func TestBrazoOffNoLlamaPeroCuenta(t *testing.T) {
	motor := &motorEspia{respuesta: respuestaElige}
	s, e := servidorDecididor(t, motor, 100)
	pid := pidDeBrazo(t, memory.BrazoOff)
	ctx := ctxConProyecto("p1", pid)

	res := s.exigirSkillSiCorresponde(ctx, "brazo off control del decididor en go",
		memory.RecallResult{}, nil, arsenalDecididor())

	if res.ExigenciaSkill != nil {
		t.Errorf("el brazo off (control) no recibe tratamiento, salió %+v", res.ExigenciaSkill)
	}
	if n := motor.llamadas.Load(); n != 0 {
		t.Errorf("el brazo off no llama al motor, hubo %d", n)
	}
	if fila := filaDecididorAB(t, e, pid, memory.BrazoOff); fila.Oportunidades != 1 || fila.Exigencias != 0 {
		t.Errorf("el off cuenta la ocasión sin exigir: esperaba oport=1 exig=0, obtuve %+v", fila)
	}
}
