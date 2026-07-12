package memory

import (
	"testing"
	"time"
)

// N4 — el ranker del recall NO puede alimentarse de su propia salida.
//
// bumpAccess escribe last_accessed y access_count sobre lo que el recall ACABA DE DEVOLVER. Si esas
// columnas rankean, el lazo se cierra: lo mostrado sube de rango ⇒ se vuelve a mostrar ⇒ sube más.
// La distinción: EXÓGENO (created_at — el ranker no lo puede cambiar) vs ENDÓGENO (last_accessed,
// access_count — los escribe el ranker).

func iso(t time.Time) string { return t.Format(time.RFC3339) }

// N4.a / R1-R2 — La recencia mide NOVEDAD, no "cuándo te lo mostré".
//
// Se aísla la señal de recencia (ambas con el mismo access_count) para que el veredicto dependa
// SÓLO de ella. La vieja lleva last_accessed = ahora, o sea: el ranker la mostró recién. Con el
// código anterior (effectiveRecency = last_accessed ?? created_at) eso la volvía "la más reciente"
// y le ganaba a una escrita ayer — el ranker premiando su propia salida.
func TestRecencyRanksByCreationNotByAccess(t *testing.T) {
	now := testNow()
	vieja := candidate{
		id:           "vieja",
		createdAt:    iso(now.AddDate(0, 0, -200)), // escrita hace 200 días
		lastAccessed: iso(now),                     // ...pero el ranker la mostró RECIÉN
	}
	nueva := candidate{
		id:        "nueva",
		createdAt: iso(now.AddDate(0, 0, -1)), // escrita ayer, nunca mostrada
	}

	scored := scoreCandidates([]candidate{vieja, nueva}, nil, nil, nil, nil, now)
	if scored[0].id != "nueva" {
		t.Errorf("una memoria vieja recién MOSTRADA no es 'reciente': debe ganar la escrita ayer. Obtuve %s primero", scored[0].id)
	}

	// Y que quede explícito el bug que se corrige: la señal vieja (last_accessed) SÍ la ponía arriba.
	if !(effectiveRecency(vieja) > effectiveRecency(nueva)) {
		t.Fatal("precondición: con el criterio anterior (last_accessed) la vieja parecía la más reciente")
	}
}

// La frecuencia NO desaparece: 9 accesos (aunque viejos) siguen valiendo más que ninguno. Lo que se
// rompe es el lock-in (la ventaja se erosiona), no la señal. Este test fija esa frontera para que un
// cambio futuro no la borre de más.
func TestFrequencyStillCountsEvenIfOld(t *testing.T) {
	now := testNow()
	usada := candidate{id: "usada", createdAt: iso(now.AddDate(0, 0, -200)), accessCount: 9}
	nunca := candidate{id: "nunca", createdAt: iso(now.AddDate(0, 0, -200))}

	if !(accessRate(usada, now) > accessRate(nunca, now)) {
		t.Error("a igual edad, la que se usó debe seguir teniendo más tasa que la que nunca se usó")
	}
}

// N4.b — La frecuencia es una TASA, no un total acumulado: a igual cantidad de accesos, la
// observación MÁS VIEJA vale menos. Es lo que hace que la ventaja SE EROSIONE si deja de usarse
// (integrador con fuga, no acumulador desbocado).
func TestFrequencyDecaysWithAge(t *testing.T) {
	now := testNow()
	const accesos = 10

	vieja := candidate{id: "vieja", createdAt: iso(now.AddDate(0, 0, -200)), accessCount: accesos}
	joven := candidate{id: "joven", createdAt: iso(now.AddDate(0, 0, -2)), accessCount: accesos}

	rVieja := accessRate(vieja, now)
	rJoven := accessRate(joven, now)
	if !(rJoven > rVieja) {
		t.Errorf("a igual access_count, la joven debe tener MAYOR tasa (joven=%.4f vieja=%.4f)", rJoven, rVieja)
	}
}

// N4.d — EL LAZO SE CORTA: una observación muy accedida hace mucho pierde la ventaja de frecuencia
// contra una poco accedida pero RECIENTE. Con el access_count crudo (que sólo sube y nunca baja) la
// vieja ganaría para siempre: ése era el lock-in.
func TestRichGetRicherLoopIsBroken(t *testing.T) {
	now := testNow()

	// La "rica": se recuperó 20 veces... hace 300 días. Desde entonces, nada.
	rica := candidate{id: "rica", createdAt: iso(now.AddDate(0, 0, -300)), accessCount: 20}
	// La "nueva": apenas 2 usos, pero de esta semana.
	nueva := candidate{id: "nueva", createdAt: iso(now.AddDate(0, 0, -3)), accessCount: 2}

	if accessRate(rica, now) >= accessRate(nueva, now) {
		t.Errorf("el lock-in sigue vivo: 20 accesos de hace 300 días no deben pesar más que 2 de esta semana (rica=%.4f nueva=%.4f)",
			accessRate(rica, now), accessRate(nueva, now))
	}

	// Con el contador CRUDO (el comportamiento viejo) la rica ganaba: dejémoslo explícito, para que
	// se vea que el fix cambia el ORDEN y no sólo la magnitud.
	if !(rica.accessCount > nueva.accessCount) {
		t.Fatal("precondición: con el contador crudo la 'rica' ganaba")
	}
}

// R6 — el arreglo "obvio" NO sirve: freqRank es un RANGO, y toda transformación MONÓTONA del
// contador (p. ej. log) conserva el orden ⇒ rank(log(x)) == rank(x) ⇒ no cambia NADA. Este test fija
// esa lección: lo que rompe el lock-in es meter el TIEMPO en la cuenta, no amortiguar la magnitud.
func TestMonotonicDampeningWouldNotHaveHelped(t *testing.T) {
	now := testNow()
	rica := candidate{id: "rica", createdAt: iso(now.AddDate(0, 0, -300)), accessCount: 20}
	nueva := candidate{id: "nueva", createdAt: iso(now.AddDate(0, 0, -3)), accessCount: 2}

	// Un log() habría dejado el MISMO orden que el contador crudo (la rica arriba)...
	if !(rica.accessCount > nueva.accessCount) {
		t.Fatal("precondición")
	}
	// ...mientras que la TASA lo INVIERTE, que es justamente el punto.
	if !(accessRate(nueva, now) > accessRate(rica, now)) {
		t.Error("la tasa debe INVERTIR el orden respecto del contador crudo; si no, no rompe el lock-in")
	}
}

// N4.c — la tasa no explota con edad ≈ 0 (el +1 del denominador).
func TestAccessRateDoesNotExplodeOnBrandNew(t *testing.T) {
	now := testNow()
	recien := candidate{id: "r", createdAt: iso(now), accessCount: 1}
	r := accessRate(recien, now)
	if r <= 0 || r > 1.001 {
		t.Errorf("una observación recién creada con 1 acceso debe dar una tasa finita y acotada (~1.0), obtuve %v", r)
	}
}

// Sin accesos, la tasa es 0 (no se penaliza ni se premia).
func TestAccessRateZeroWithoutAccess(t *testing.T) {
	now := testNow()
	if r := accessRate(candidate{createdAt: iso(now.AddDate(0, 0, -10))}, now); r != 0 {
		t.Errorf("sin accesos la tasa debe ser 0, obtuve %v", r)
	}
}

// Degradación segura: un created_at ilegible ⇒ edad 0 ⇒ tasa = contador crudo (comportamiento
// previo para esa fila). No rompe ni descarta la observación.
func TestAccessRateDegradesSafelyOnBadTimestamp(t *testing.T) {
	now := testNow()
	roto := candidate{id: "x", createdAt: "no-es-una-fecha", accessCount: 5}
	if r := accessRate(roto, now); r != 5 {
		t.Errorf("con timestamp ilegible la tasa debe caer al contador crudo (5), obtuve %v", r)
	}
}
