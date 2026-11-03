package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"

	_ "modernc.org/sqlite" // driver para leer la fila CRUDA de skill_usage (R1/R3/R5)
)

// Invariantes del spec skills-medir-cargas del lado del hook `skill-loaded`.

// LA DECISIÓN DE SI AL HOOK LE TOCA ESCRIBIR vive en un solo lugar (leerEventoSkillLoaded), para
// poder probarla sin base. Cubre R4 (sólo la tool Skill con nombre no vacío) y el kill switch (D5).
//
// R4 tiene DOS mitades y el fixture tiene que atacar cada una. La mitad `skill != ""` ya la cubren los
// casos de skill vacía. La mitad `tool_name == "Skill"` la cubre el caso "otra tool con clave skill":
// una tool ajena que SÍ trae `tool_input.skill`. Sin ese caso, quitar el arm de tool_name queda verde.
//
// Sabotaje que la pone en rojo: quitar el arm `tool_name == "Skill"` de skillLoadedAplica; la tool
// ajena con clave `skill` pasaría a aplicar y el caso "otra tool con clave skill" fallaría.
// arnes: archivo="cmd/musubi/skill_loaded.go" de="return in.ToolName == \"Skill\" && strings.TrimSpace(in.ToolInput.Skill) != \"\"" a="return strings.TrimSpace(in.ToolInput.Skill) != \"\""
func TestLeerEventoSkillLoadedDecide(t *testing.T) {
	casos := []struct {
		nombre string
		json   string
		env    string // valor de MUSUBI_SKILLLOAD_HOOK; "" = sin setear
		quiero bool
	}{
		{"carga de skill con nombre", `{"tool_name":"Skill","tool_input":{"skill":"sdd-flow"}}`, "", true},
		{"otra tool es no-op", `{"tool_name":"Read","tool_input":{"file_path":"x.go"}}`, "", false},
		{"otra tool con clave skill es no-op (R4 por tool_name)", `{"tool_name":"Read","tool_input":{"skill":"sdd-flow"}}`, "", false},
		{"skill vacía es no-op", `{"tool_name":"Skill","tool_input":{"skill":""}}`, "", false},
		{"skill sólo espacios es no-op", `{"tool_name":"Skill","tool_input":{"skill":"   "}}`, "", false},
		{"kill switch apaga aun con carga válida", `{"tool_name":"Skill","tool_input":{"skill":"sdd-flow"}}`, "0", false},
		{"json inválido no aplica (y no entra en pánico)", `{roto`, "", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			// t.Setenv aísla la variable por subtest y la restaura al terminar.
			if c.env != "" {
				t.Setenv("MUSUBI_SKILLLOAD_HOOK", c.env)
			} else {
				t.Setenv("MUSUBI_SKILLLOAD_HOOK", "")
			}
			_, aplica := leerEventoSkillLoaded(strings.NewReader(c.json))
			if aplica != c.quiero {
				t.Errorf("aplica = %v, esperaba %v", aplica, c.quiero)
			}
		})
	}
}

// LA CARGA QUEDA REGISTRADA con kind loaded y suma por upsert. El daemon crea y migra la base; el hook
// la abre SIN arranque (como precheck) y hace el upsert. Acá se verifica sólo que la carga se cuenta
// como `loaded` y NO como resolución; el project_id (R1), el evidence vacío (R3) y que los args NO se
// persistan (R5) los asserta TestEscribirSkillCargadaNoFiltraArgs leyendo la fila CRUDA —SkillUsage()
// agrega por skill y oculta esas dos columnas, así que acá no se podrían ver—.
func TestEscribirSkillCargadaDejaLaFila(t *testing.T) {
	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatalf("ensureWorkspace: %v", err)
	}
	eng, err := memory.NewDbEngine(dir) // migra el esquema, como el daemon
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	eng.Close()

	in := skillLoadedInput{ToolName: "Skill"}
	in.ToolInput.Skill = "sdd-flow"
	if err := escribirSkillCargada(dir, in); err != nil {
		t.Fatalf("escribirSkillCargada: %v", err)
	}

	// Se lee con EL MISMO scope que usó la escritura (resolveProjectID sobre el mismo dir y config),
	// así el test no depende de cómo se derive el project_id, sólo de que coincida consigo mismo.
	cfg, _ := config.Load(dir)
	proj := resolveProjectID(cfg, dir)
	eng2, err := memory.NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("reabrir sin arranque: %v", err)
	}
	defer eng2.Close()
	ctx := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: proj})
	rows, err := eng2.SkillUsage(ctx)
	if err != nil {
		t.Fatalf("SkillUsage: %v", err)
	}
	var vista *memory.SkillUsageRow
	for i := range rows {
		if rows[i].Skill == "sdd-flow" {
			vista = &rows[i]
		}
	}
	if vista == nil {
		t.Fatalf("la carga no quedó registrada; filas: %+v", rows)
	}
	if vista.Loaded != 1 {
		t.Errorf("esperaba loaded=1, obtuve %d", vista.Loaded)
	}
	if vista.Resolved != 0 {
		t.Errorf("una carga no es una resolución: esperaba resolved=0, obtuve %d", vista.Resolved)
	}
}

// LOS ARGS NO SE PERSISTEN (R5), la fila va al project_id del cwd (R1) y con evidence vacío (R3).
// Estos tres invariantes NO los ve SkillUsage() —agrega por skill y oculta project_id/evidence—, así
// que el custodio lee la fila CRUDA de skill_usage. El canario entra por `tool_input.args` y pasa por
// el decodificador real (leerEventoSkillLoaded): hoy el struct no tiene campo Args y json lo descarta,
// pero si alguien lo agregara y lo concatenara al nombre, el canario aparecería en la columna `skill`
// o en los bytes de la base. Un struct tipado a mano no tendría dónde poner los args, así que el
// único camino que prueba R5 de verdad es el que arranca en el JSON.
//
// Cada invariante tiene su sabotaje en UN solo sitio, y cada uno va en su propia ancla:
// Sabotaje R1 (project_id del cwd): con el project_id vacío la fila matchea cualquier scope y deja
// de estar acotada al tenant; `proj[:0]` lo vacía SIN dejar `proj` sin usar (un `""` pelado no
// compilaría —«declared and not used»— y el rojo sería por build roto, no por la guarda).
// arnes: archivo="cmd/musubi/skill_loaded.go" de="ProjectID: proj," a="ProjectID: proj[:0],"
// Sabotaje R3 (evidence vacío): con evidence no vacío, una carga mentiría que vino de una resolución.
// arnes: archivo="cmd/musubi/skill_loaded.go" de="Evidence:  \"\"," a="Evidence:  \"alcance\","
// Sabotaje R5 (args no se persisten): si el tag del nombre leyera la clave `args`, el canario que el
// test mete en `tool_input.args` entraría por el nombre y terminaría en la base.
// arnes: archivo="cmd/musubi/skill_loaded.go" de="Skill string `json:\"skill\"`" a="Skill string `json:\"args\"`"
func TestEscribirSkillCargadaNoFiltraArgs(t *testing.T) {
	const canario = "CANARIO-ARGS-NO-DEBE-PERSISTIR"

	dir := t.TempDir()
	if err := ensureWorkspace(dir); err != nil {
		t.Fatalf("ensureWorkspace: %v", err)
	}
	eng, err := memory.NewDbEngine(dir) // migra el esquema, como el daemon
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	eng.Close()

	// El evento ENTRA por el decodificador real con `args` en el payload: ese es el único camino por el
	// que un Args podría llegar a escribirse. Hoy aplica y los args se descartan.
	js := `{"tool_name":"Skill","tool_input":{"skill":"sdd-flow","args":"` + canario + `"}}`
	in, aplica := leerEventoSkillLoaded(strings.NewReader(js))
	if !aplica {
		t.Fatalf("un evento Skill con args válidos debería aplicar")
	}
	if err := escribirSkillCargada(dir, in); err != nil {
		t.Fatalf("escribirSkillCargada: %v", err)
	}

	cfg, _ := config.Load(dir)
	proj := resolveProjectID(cfg, dir)
	if proj == "" {
		t.Fatal("project_id resuelto vacío: el custodio de R1 no mediría nada")
	}

	// Lectura CRUDA: SkillUsage() agrega por skill y no expone project_id ni evidence, así que un
	// project_id vacío (que matchea cualquier scope por scope.go) o un evidence contaminado se le
	// escapan. Acá se leen las columnas tal cual quedaron en la base.
	dbPath := filepath.Join(dir, config.DirName, config.DBFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("abrir base: %v", err)
	}
	defer db.Close()
	filas, err := db.Query(`SELECT skill, project_id, evidence, kind, n FROM skill_usage WHERE kind = 'loaded'`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer filas.Close()
	var n int
	for filas.Next() {
		n++
		var skill, projectID, evidence, kind string
		var cuenta int
		if err := filas.Scan(&skill, &projectID, &evidence, &kind, &cuenta); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if skill != "sdd-flow" { // R5: el nombre exacto, sin args pegados
			t.Errorf("skill = %q, esperaba \"sdd-flow\" (¿se concatenó algo al nombre?)", skill)
		}
		if projectID != proj { // R1: acotada al tenant del cwd, NO vacía
			t.Errorf("project_id = %q, esperaba %q", projectID, proj)
		}
		if evidence != "" { // R3: una carga no viene de una resolución
			t.Errorf("evidence = %q, esperaba vacío", evidence)
		}
		if strings.Contains(skill+projectID+evidence, canario) { // R5: ninguna columna lo contiene
			t.Errorf("el canario de args se filtró: skill=%q project_id=%q evidence=%q", skill, projectID, evidence)
		}
	}
	if err := filas.Err(); err != nil {
		t.Fatalf("iterar filas: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperaba exactamente 1 fila loaded, hay %d", n)
	}

	// Defensa extra de R5: el canario no aparece en NINGÚN byte de la base (ni en el -wal), aunque se
	// hubiera filtrado a una columna que el SELECT de arriba no mira.
	var crudo []byte
	for _, p := range []string{dbPath, dbPath + "-wal"} {
		if b, err := os.ReadFile(p); err == nil {
			crudo = append(crudo, b...)
		}
	}
	// Control positivo: el MISMO barrido SÍ encuentra el nombre de la skill, que sabemos escrito. Sin
	// esto, un dbPath equivocado o una base vacía dejarían el barrido mirando nada, y el canario
	// «ausente» no probaría nada —pasaría en verde por no leer, no por no filtrarse—.
	if !bytes.Contains(crudo, []byte("sdd-flow")) {
		t.Fatal("el barrido no encontró ni el nombre de la skill: no está leyendo la base, el control de R5 no mediría")
	}
	if bytes.Contains(crudo, []byte(canario)) {
		t.Errorf("el canario de args quedó en los bytes de la base: los args se persistieron (R5)")
	}
}

// BEST-EFFORT: sin base, escribirSkillCargada devuelve ErrBaseAusente SIN entrar en pánico, para que
// runSkillLoaded lo pueda tragar y salir 0 sin romper al agente (R6).
func TestEscribirSkillCargadaSinBaseNoRompe(t *testing.T) {
	dir := t.TempDir() // vacío: no hay .musubi
	in := skillLoadedInput{ToolName: "Skill"}
	in.ToolInput.Skill = "sdd-flow"
	err := escribirSkillCargada(dir, in)
	if !errors.Is(err, memory.ErrBaseAusente) {
		t.Fatalf("sin base esperaba ErrBaseAusente (best-effort), obtuve %v", err)
	}
}

// LA BASE AUSENTE ES SILENCIO, CUALQUIER OTRO ERROR SE DICE (R6). Es la otra mitad de best-effort: no
// sólo salir 0 (eso lo cubre el test de arriba), sino no vomitar un error que no es tal cuando el
// proyecto simplemente no usa Musubi. ruidoDeSkillLoaded aísla esa decisión para medirla sin montar
// una base; se le pasa ErrBaseAusente ENVUELTO porque así es como llega desde la capa de memoria.
//
// Sabotaje que la pone en rojo: cambiar el `||` por `&&` en el guard de ruidoDeSkillLoaded; la base
// ausente dejaría de ser silencio y pasaría a gritar un error que no lo es.
// arnes: archivo="cmd/musubi/skill_loaded.go" de="err == nil || errors.Is(err, memory.ErrBaseAusente)" a="err == nil && errors.Is(err, memory.ErrBaseAusente)"
func TestSkillLoadedBaseAusenteEsSilencio(t *testing.T) {
	if ruido := ruidoDeSkillLoaded(nil); ruido != "" {
		t.Errorf("sin error no hay nada que decir, obtuve %q", ruido)
	}
	if ruido := ruidoDeSkillLoaded(fmt.Errorf("al abrir: %w", memory.ErrBaseAusente)); ruido != "" {
		t.Errorf("la base ausente es silencio (el proyecto no usa Musubi), obtuve %q", ruido)
	}
	otro := errors.New("disco lleno")
	if ruido := ruidoDeSkillLoaded(otro); !strings.Contains(ruido, "disco lleno") {
		t.Errorf("cualquier otro error SÍ se dice, obtuve %q", ruido)
	}
}
