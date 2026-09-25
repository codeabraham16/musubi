package main

import (
	"fmt"
	"testing"
)

// DOS SESIONES ABIERTAS A LA VEZ NO SE PISAN LOS AVISOS.
//
// Las pruebas de cada aviso usaban UNA sesión, y con una sola la casilla única funciona: el defecto
// sólo aparece cuando dos terminales se turnan en el mismo repo, que es el uso normal. Medido el
// 2026-09-25: el aviso de «bajá lo durable» salió hasta 173 veces en una sesión y 375 de 375
// inyecciones llegaron justo después de un turno de OTRA sesión; el de conflictos, 336 de 372.

// Se mira EN QUÉ TURNOS sale, no cuántas veces: con un contador compartido entre las dos sesiones el
// total puede dar igual (los disparos se reparten) y una guarda que sólo contara quedaría hueca.
//
// Sabotaje que la hace fallar: que las dos sesiones compartan el contador de turnos.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tkey := metaLoopTurnsSession + \":\" + sessionID\n\tturns, _ := readIntMeta(store, key)"
// arnes: a="\tkey := metaLoopTurnsSession\n\tturns, _ := readIntMeta(store, key)"
func TestElAvisoDurableCuentaLosTurnosDeCadaSesionConOtraIntercalada(t *testing.T) {
	store := newFakeTurnStore()
	turnos := map[string][]int{}
	for i := 1; i <= 30; i++ {
		for _, sesion := range []string{"A", "B"} {
			if buildDurableNudge(store, sesion, 5) != "" {
				turnos[sesion] = append(turnos[sesion], i)
			}
		}
	}
	for _, sesion := range []string{"A", "B"} {
		if got := fmt.Sprint(turnos[sesion]); got != "[5 10 15 20 25 30]" {
			t.Errorf("la sesión %s, intercalada con otra, recibió el aviso en sus turnos %s; con umbral 5 tienen que ser [5 10 15 20 25 30]", sesion, got)
		}
	}
}

// Sabotaje que la hace fallar: volver a la casilla única de «id\x00payload».
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\treturn marcarUnaVezPorSesion(store, key, sessionID, payload)\n"
// arnes: a="\twant := sessionID + \"\\x00\" + payload\n\tif prev, ok, _ := store.GetMeta(key); ok && prev == want {\n\t\treturn false\n\t}\n\t_ = store.SetMeta(key, want)\n\treturn true\n"
func TestLasSuperficiesPorTurnoNoSeRepitenConOtraSesionIntercalada(t *testing.T) {
	store := newFakeTurnStore()
	// El primer turno de cada sesión con 3 conflictos pendientes avisa: es nuevo para esa sesión.
	for _, sesion := range []string{"A", "B"} {
		if !turnSurfaceChanged(store, metaConflictsInjected, sesion, "3") {
			t.Fatalf("la sesión %s no recibió el aviso la primera vez", sesion)
		}
	}
	// Los turnos siguientes, con la misma cuenta, no: aunque la otra sesión haya pasado en el medio.
	for i := 0; i < 5; i++ {
		for _, sesion := range []string{"A", "B"} {
			if turnSurfaceChanged(store, metaConflictsInjected, sesion, "3") {
				t.Fatalf("turno %d: la sesión %s recibió otra vez el mismo aviso porque la otra pasó en el medio", i, sesion)
			}
		}
	}
	// Y si la cuenta cambia, esa sesión sí se entera.
	if !turnSurfaceChanged(store, metaConflictsInjected, "A", "4") {
		t.Error("la cuenta pasó de 3 a 4 y la sesión A no recibió el aviso")
	}
}
