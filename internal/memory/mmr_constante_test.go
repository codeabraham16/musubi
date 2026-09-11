package memory

import "testing"

// MMR NO ES INVARIANTE A SUMARLE UNA CONSTANTE A TODOS LOS SCORES, Y ESO HACE FALTA SABERLO.
//
// Parece un detalle de implementación y es lo que arruinó una medición entera el 2026-09-11. El
// barrido de pesos del banco (internal/recalleval) daba que apagar `recencia`, `frecuencia` o
// `importancia` mejoraba el nDCG en +0.0200 — las tres exactamente igual, hasta el cuarto decimal.
//
// No era una propiedad de esas señales. En el fixture las tres están PLANAS: todos los documentos
// tienen el mismo valor, así que `denseRankBy` les da a todos el rango 0 y su término es una
// CONSTANTE idéntica para cada candidato. Apagarlas no saca información — saca una constante.
//
// Y una constante SÍ mueve el resultado, por esto: `normalizeScores` divide por el MÁXIMO, no
// min-max. Restarle `c` a todos los scores no desplaza el resultado, lo ESTIRA:
//
//	sin constante:  (a-c)/(b-c)   ≠   a/b   :con constante
//
// Con la relevancia normalizada más separada, MMR diversifica menos. O sea que el barrido estaba
// midiendo CUÁNTO MMR se apagaba, no cuánto aportaba cada señal. Un número bien calculado
// contestando otra pregunta — la forma que este repo ya tiene nombrada.
//
// Esta guarda fija la propiedad para que el próximo que mida no la descubra a los 24 minutos de
// corrida. No dice que dividir por el máximo esté mal: dice que NO es invariante, que es lo que hay
// que tener delante al comparar dos configuraciones del ranker.

// A — DIVIDIR POR EL MÁXIMO ESTIRA, NO DESPLAZA.
func TestNormalizeScoresNoEsInvarianteAUnaConstante(t *testing.T) {
	base := []scoredCandidate{
		{candidate: candidate{id: "a"}, score: 0.30},
		{candidate: candidate{id: "b"}, score: 0.20},
		{candidate: candidate{id: "c"}, score: 0.10},
	}
	// La MISMA información relativa, más una constante idéntica para todos: es exactamente lo que
	// agrega (o saca) una señal plana del RRF.
	const c = 0.10
	conConstante := make([]scoredCandidate, len(base))
	for i, s := range base {
		conConstante[i] = s
		conConstante[i].score = s.score + c
	}

	sinC := normalizeScores(base)
	conC := normalizeScores(conConstante)

	if len(sinC) != len(conC) {
		t.Fatalf("distinto largo: %d vs %d", len(sinC), len(conC))
	}
	// El primero es el máximo en los dos casos, así que normaliza a 1 en los dos: no distingue nada.
	// Lo que decide es el ÚLTIMO, donde el estiramiento se ve entero.
	if sinC[2] == conC[2] {
		t.Fatalf("normalizeScores resultó invariante a la constante (%.6f en los dos casos). "+
			"Si eso es cierto, toda la explicación del barrido de pesos es falsa y hay que rehacerla.", sinC[2])
	}
	// Y EN LA DIRECCIÓN QUE IMPORTA: con la constante puesta, el mínimo queda MÁS ALTO, o sea la
	// relevancia queda más JUNTA. Sacar la constante SEPARA. Si la dirección fuera la otra, el
	// diagnóstico del barrido estaría invertido.
	if !(conC[2] > sinC[2]) {
		t.Errorf("con la constante el mínimo normalizado tendría que subir (relevancia más junta): sin=%.6f con=%.6f", sinC[2], conC[2])
	}
}

// B — Y ESO LLEGA HASTA EL ORDEN QUE DEVUELVE MMR, POR EL CAMINO REAL.
//
// La propiedad de arriba sólo importa si se propaga. Acá se ejercita `diversify` —el consumidor—
// con DOS listas que tienen EXACTAMENTE las mismas diferencias de score (0.010 entre vecinos) y
// distinto nivel absoluto. Si MMR dependiera sólo del orden y de las distancias relativas, las dos
// tendrían que salir igual.
//
// No salen igual, y por eso un barrido de pesos que no apague MMR mide cuánto MMR se apagó.
func TestLaConstanteLlegaHastaElOrdenDeMMR(t *testing.T) {
	e := mmrEngine(t)
	setupClones(t, e) // A y B clones (coseno 0.98); C aporta información nueva

	conNivelAlto := []scoredCandidate{ // lo que produce un RRF con varias señales planas sumando
		{candidate: candidate{id: "A"}, score: 0.100},
		{candidate: candidate{id: "B"}, score: 0.090},
		{candidate: candidate{id: "C"}, score: 0.080},
	}
	conNivelBajo := []scoredCandidate{ // las MISMAS diferencias, sin esa constante
		{candidate: candidate{id: "A"}, score: 0.025},
		{candidate: candidate{id: "B"}, score: 0.015},
		{candidate: candidate{id: "C"}, score: 0.005},
	}

	alto := mmrIDs(e.diversify(conNivelAlto, 0.75))
	bajo := mmrIDs(e.diversify(conNivelBajo, 0.75))

	iguales := true
	for i := range alto {
		if alto[i] != bajo[i] {
			iguales = false
		}
	}
	if iguales {
		t.Errorf("las mismas diferencias con distinto nivel absoluto dieron el MISMO orden (%v). "+
			"Si de verdad no cambia, entonces apagar una señal PLANA del RRF no puede mover ninguna "+
			"métrica, y los +0.0200 del primer barrido de pesos necesitan otra explicación.", alto)
	}
	t.Logf("mismas diferencias, distinto nivel: con la constante %v · sin ella %v", alto, bajo)
}
