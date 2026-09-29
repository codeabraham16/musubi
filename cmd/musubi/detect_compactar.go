package main

import (
	"fmt"
	"os"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// detect_compactar.go — DESPUÉS DE COMPACTAR, MUSUBI DEVUELVE LA MEMORIA QUE EL RESUMEN PERDIÓ.
//
// Cuando Claude Code compacta, cambia la conversación por un resumen y dispara SessionStart con
// source «compact» y el MISMO session_id. Lo que Musubi había inyectado en la sesión se fue con la
// conversación, pero el delta de la sesión seguía diciendo que el agente lo tenía, y el turno no lo
// repetía. Medido el 2026-09-26 en davantis-1: 147 compactaciones contra 5 arranques, ninguna seguida
// de un SessionStart de Musubi —setup instalaba el hook sólo con matcher «startup»—, y el delta
// escondió 132 de los 220 ids que el recall por turno volvió a encontrar (60 %).
//
// ESTO NO ES EL ARRANQUE. No refresca manuales, no ofrece generar skills, no corre el bloque
// cognitivo ni el de salud, y no trae el priming del proyecto: en medio de una sesión eso ya está en
// el resumen o sobra. Hace tres cosas, en este orden:
//
//  1. vacía el delta de ESTA sesión (las demás conservan el suyo);
//  2. olvida las marcas «ya avisado en esta sesión» de la fase, el lote y los conflictos, que también
//     se fueron con la conversación: el próximo turno que tenga algo que decir lo vuelve a decir;
//  3. vuelve a buscar memoria sobre el ÚLTIMO pedido sustantivo de la sesión (lo que guardó
//     recordarPedido) con el MISMO recall del turno, y la emite SIN mostrar el pedido: el resumen de
//     Claude Code ya lo trae textual. Tampoco va la línea «busqué … por …» del corrector de tipeo, que
//     es del turno, para quien acaba de tipear (decisión 2 del dueño); la consulta sí sale corregida.
//
// Sin pedidos guardados calla, pero el delta queda limpio igual: el próximo turno trae su memoria.

// surfaceCompactRecall es la superficie del ledger de la memoria que vuelve tras compactar.
const surfaceCompactRecall = "compact_recall"

// marcasQueOlvidaLaCompactacion son las marcas por sesión de lo que el turno avisa una vez mientras no
// cambie (turnSurfaceChanged). No están la alerta de presupuesto ni la directiva de brevedad
// (metaBudgetAlerted, metaBrevityInjected): el resumen también se las lleva, y si vuelven o no es
// otra decisión.
var marcasQueOlvidaLaCompactacion = []string{metaPhaseInjected, metaBatchInjected, metaConflictsInjected}

// detectOutputDeCompactacion abre la memoria del proyecto y arma la salida del hook para una
// compactación. Best-effort como todo hook: si la memoria no abre, lo dice por stderr y calla.
func detectOutputDeCompactacion(root string, cfg config.Config, sessionID string) string {
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi detect: memoria no disponible tras compactar: %v\n", err)
		return ""
	}
	defer engine.Close()
	// El proyecto propio y el embebedor salen de las MISMAS funciones que usa el hook del turno: este
	// recall tiene que ser el del turno, no un pariente que envejece por su lado.
	propio := resolveProjectID(cfg, root)
	return buildHookOutputDeCompactacion(engine, cfg.Loop, cfg.Memory, sessionID, propio, embebedorDelHook(cfg, root, engine))
}

// buildHookOutputDeCompactacion es la rama «compact» del hook de arranque. Ver el comentario del
// archivo: limpia, olvida, y vuelve a buscar sobre el último pedido.
func buildHookOutputDeCompactacion(store turnStore, loopCfg config.LoopConfig, memCfg config.MemoryConfig, sessionID, propio string, embedder embedding.Provider) string {
	if store == nil {
		return ""
	}
	clearDeltaState(store, sessionID)
	for _, key := range marcasQueOlvidaLaCompactacion {
		olvidarMarcaDeSesion(store, key, sessionID)
	}
	if !loopCfg.PerTurnRecall {
		return ""
	}
	ps := leerPedidos(store, sessionID)
	if len(ps) == 0 {
		return ""
	}
	ultimo := ps[len(ps)-1].Texto
	bloque := buildTurnRecall(store, parametrosDelTurno{
		sesion:               sessionID,
		prompt:               ultimo,
		propio:               propio,
		presupuesto:          loopCfg.RecallBudget,
		delta:                loopCfg.DeltaInjection,
		memCfg:               memCfg,
		embebedor:            embedder,
		otrosModo:            loopCfg.RecallOtrosProyectos,
		otrosMax:             loopCfg.RecallOtrosMax,
		sinAvisoDeCorreccion: true,
	})
	return assembleAccounted(store, "SessionStart", sessionID, []accountedBlock{
		{surface: surfaceCompactRecall, text: bloque},
	})
}

// ── El aviso de salud: un repo sin el hook de compactación ───────────────────────────────────

// metaAvisoSinCompact marca que el aviso ya salió en este proyecto (la base es del proyecto). El
// valor es la fecha en que salió.
const metaAvisoSinCompact = "arranque_aviso_sin_compact"

// surfaceAvisoSinCompact es la superficie del ledger del aviso.
const surfaceAvisoSinCompact = "startup_aviso_compact"

// textoAvisoSinCompact es el aviso. Dice qué se pierde y cómo se arregla, y que no se repite.
const textoAvisoSinCompact = "[Musubi — salud] Este repo no tiene el hook SessionStart «compact» de Musubi: después de compactar, " +
	"la memoria que el resumen pierde no vuelve. Lo instala `musubi setup` en el repo (con el binario nuevo: van juntos), " +
	"o `musubi agente instalar` para el plugin. Este aviso sale una sola vez por proyecto."

// buildAvisoSinCompact devuelve el aviso UNA sola vez por proyecto, en el arranque, cuando nada
// ata el hook de Musubi a la compactación: ni el settings del repo ni el plugin instalado. La marca
// se pone sólo cuando el aviso sale; si el repo está cubierto no se escribe nada, y si deja de
// estarlo, avisa entonces.
func buildAvisoSinCompact(store metaStore, dirs ...string) string {
	if store == nil {
		return ""
	}
	if v, ok, _ := store.GetMeta(metaAvisoSinCompact); ok && v != "" {
		return ""
	}
	if compactCubierto(dirs...) {
		return ""
	}
	_ = store.SetMeta(metaAvisoSinCompact, time.Now().Format("2006-01-02"))
	return textoAvisoSinCompact
}
