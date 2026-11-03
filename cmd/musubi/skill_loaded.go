package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// skill_loaded.go implementa 'musubi skill-loaded --hook-mode': el hook PostToolUse atado a la tool
// `Skill`, que registra que el agente CARGÓ una skill del camino `.claude/skills/`.
//
// POR QUÉ EXISTE. `skill_usage` contaba RESOLUCIONES del resolver de reflejos, pero la carga por la
// tool `Skill` —donde decide el consumidor— no la veía nadie: el spec `skills-que-se-miden` declaró
// ese límite a propósito (su §del límite), porque `tool_invocations` no guarda argumentos. Así la
// pregunta «¿el agente cargó la skill X?» sólo se contestaba leyendo transcripts offline. Este hook
// la convierte en una señal viva y model-free: un `Kind=loaded` más en la tabla que ya existe.
//
// QUÉ GUARDA Y QUÉ NO. Sólo el NOMBRE de la skill (público del catálogo). Los `args` que el agente
// le pasó NO se tocan: eso mantiene intacta la garantía de privacidad de `tool_invocations`. Puro
// reflejo, best-effort: cualquier fallo sale 0 y jamás demora al agente.

// skillLoadedInput es el subconjunto del JSON de stdin de PostToolUse que usamos. La clave del
// argumento es `skill` porque así se llama el parámetro de la tool `Skill` (análogo a cómo precheck
// lee `tool_input.file_path` de `Read`). `args` existe en el payload y se ignora a propósito (R5).
type skillLoadedInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Skill string `json:"skill"`
	} `json:"tool_input"`
	SessionID string `json:"session_id"`
}

// skillLoadHabilitado: el hook se apaga con MUSUBI_SKILLLOAD_HOOK=0, kill switch simétrico con el
// MUSUBI_CODEGRAPH_HOOK de precheck. Cualquier otro valor —o ausente— lo deja encendido: la feature
// es opt-in por instalación, y el env da un corte rápido sin editar settings.json.
func skillLoadHabilitado() bool {
	return os.Getenv("MUSUBI_SKILLLOAD_HOOK") != "0"
}

// skillLoadedAplica: el hook sólo escribe cuando el agente cargó una skill con la tool `Skill` y el
// nombre no está vacío. Para cualquier otra tool, o skill vacía, es no-op (R4). El matcher de
// settings.json ya acota a `Skill`; esta comprobación es la defensa en profundidad que no depende de
// que el matcher esté bien escrito.
func skillLoadedAplica(in skillLoadedInput) bool {
	return in.ToolName == "Skill" && strings.TrimSpace(in.ToolInput.Skill) != ""
}

// leerEventoSkillLoaded lee stdin, lo decodifica y dice si al hook le toca ESCRIBIR. Es LA decisión,
// en un solo lugar (molde de leerEventoPrecheck), para poder testearla sin base. Incluye el kill
// switch: la pregunta que contesta el bool es «¿este hook tiene que registrar algo?», y
// MUSUBI_SKILLLOAD_HOOK=0 la contesta que no sin importar el payload.
func leerEventoSkillLoaded(stdin io.Reader) (skillLoadedInput, bool) {
	var in skillLoadedInput
	data, err := io.ReadAll(stdin)
	if err != nil {
		return in, false
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return in, false
	}
	return in, skillLoadHabilitado() && skillLoadedAplica(in)
}

// escribirSkillCargada abre la base y suma un conteo `loaded` para la skill.
//
// Abre con NewDbEngineSinArranque, como precheck: ni crea la base, ni migra, ni backfillea. El
// daemon ya migró el esquema; este proceso corto sólo hace un upsert diminuto. WAL + busy_timeout
// manejan la concurrencia con el daemon. El project_id sale del cwd igual que en capture
// (resolveProjectID), para que el conteo quede acotado al tenant correcto (R1).
func escribirSkillCargada(root string, in skillLoadedInput) error {
	cfg, _ := config.Load(root)
	proj := resolveProjectID(cfg, root)

	eng, err := memory.NewDbEngineSinArranque(root)
	if err != nil {
		return err // el caller distingue ErrBaseAusente (silencioso) del resto
	}
	defer eng.Close()

	// Techo de 5 s: si el daemon tuviera la base trabada más que eso, abandonamos antes que colgar
	// al agente. Un conteo perdido no sesga una TASA si las pérdidas son parejas (best-effort).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Evidence vacío: la carga NO viene de una resolución (misma forma que body_requested). La fila
	// resultante (skill, proj, "", "loaded") queda distinguible de las de resolución (R3).
	return eng.RecordSkillEvents(ctx, []memory.SkillEvent{{
		Skill:     strings.TrimSpace(in.ToolInput.Skill),
		ProjectID: proj,
		Evidence:  "",
		Kind:      memory.UsoCargada,
	}})
}

// ruidoDeSkillLoaded decide QUÉ se dice por stderr ante un error de la escritura, aislado del resto
// para poder probar la frontera de R6 sin montar una base. Devuelve "" cuando no hay nada que decir
// —ni error, o la base ausente, que sólo significa «este proyecto no usa Musubi» y no es un fallo— y
// el renglón a imprimir para cualquier otro error. El caller imprime esto y sale 0 igual: best-effort.
func ruidoDeSkillLoaded(err error) string {
	if err == nil || errors.Is(err, memory.ErrBaseAusente) {
		return ""
	}
	return fmt.Sprintf("musubi skill-loaded: %v\n", err)
}

// runSkillLoaded implementa 'musubi skill-loaded [--hook-mode]'. Sin --hook-mode es no-op (permite
// invocarlo a mano sin efecto). En hook-mode decide, y sólo si le toca abre la base y escribe.
//
// PRIMERO DECIDE, DESPUÉS ABRE (molde de precheck): un evento que no aplica no paga la apertura de la
// base. Best-effort de punta a punta: todo error va a stderr y sale 0, nunca rompe ni demora la
// sesión (R6).
func runSkillLoaded() {
	hookMode := false
	for _, arg := range os.Args[2:] {
		if arg == "--hook-mode" {
			hookMode = true
		}
	}
	if !hookMode {
		return
	}

	in, aplica := leerEventoSkillLoaded(os.Stdin)
	if !aplica {
		return
	}

	// Sólo sobre un proyecto que YA tiene memoria (ver raiz.go). Los hooks NUNCA la crean: si el
	// daemon todavía no activó el proyecto (Activar), nos callamos. Y si estamos como plugin y el
	// repo ya trae el gancho propio, se lo cedemos para no contar dos veces.
	r := raizDelProceso()
	if r.Dir == "" || r.Activar || elPluginCedeElGancho("skill-loaded") {
		return
	}

	// Qué de un fallo de la escritura es ruido —nada, si la base no está porque el proyecto no usa
	// Musubi; el renglón, para cualquier otro error— lo decide ruidoDeSkillLoaded. Imprimimos y
	// salimos 0 igual: un hook jamás rompe ni demora al agente (R6).
	if ruido := ruidoDeSkillLoaded(escribirSkillCargada(r.Dir, in)); ruido != "" {
		fmt.Fprint(os.Stderr, ruido)
	}
}
