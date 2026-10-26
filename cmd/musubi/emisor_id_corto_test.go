package main

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

// EL ENVOLTORIO DEL HOOK ACORTA EL [id:…] DE CADA VIÑETA, EN LOS CUATRO EMISORES.
//
// El «[id:…]» que el hook pega al final de cada viñeta es overhead PURO del envoltorio: NO cuenta
// contra el presupuesto del recall (ese se gasta en el gist), así que un UUID de 36 caracteres paga
// ~7 tokens por viñeta en CADA turno sin significarle nada al modelo. memory.IDCorto lo reduce a 8
// hex y el resolvedor (hydrateByIDs, expand.go) los re-expande por prefijo. Ver internal/memory/idcorto.go.
//
// SE MIDEN LOS CUATRO SITIOS, NO UNA MUESTRA. Hay dos formateadores hermanos —formatDeltaGists (el
// turno) y formatGists (el priming)— y cada uno ramifica en con-topic / sin-topic: cuatro Fprintf
// distintos emiten el id. La trampa dominante de este repo es arreglar un formateador y no el
// hermano (ver inyeccion_indirecta_test.go, I2), así que acortar en tres de cuatro dejaría un turno
// pagando el UUID entero sin que nada lo dijera.
//
// EL ID ES UN UUID CANÓNICO A PROPÓSITO. Con un id no-UUID, IDCorto devuelve el id entero y revertir
// el emisor no cambiaría la salida: un sabotaje VACUO que dejaría esta guarda en falso verde. Con un
// UUID, acortar y no-acortar producen textos distintos, así que cada sabotaje de abajo puede ponerla
// roja. El control de IDCorto al inicio del cuerpo deja explícito ese supuesto.
//
// Sabotaje: el turno con topic emite el UUID entero en vez de la ref corta.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\t\tfmt.Fprintf(&b, \"- %s(%s) %s%s%s [id:%s]\\n\", marca, topic, gist, age, suffix, memory.IDCorto(it.ID))\n"
// arnes: a="\t\t\tfmt.Fprintf(&b, \"- %s(%s) %s%s%s [id:%s]\\n\", marca, topic, gist, age, suffix, it.ID)\n"
//
// Sabotaje: el turno sin topic emite el UUID entero en vez de la ref corta.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\t\tfmt.Fprintf(&b, \"- %s%s%s%s [id:%s]\\n\", marca, gist, age, suffix, memory.IDCorto(it.ID))\n"
// arnes: a="\t\t\tfmt.Fprintf(&b, \"- %s%s%s%s [id:%s]\\n\", marca, gist, age, suffix, it.ID)\n"
//
// Sabotaje: el priming con topic emite el UUID entero en vez de la ref corta.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\t\tfmt.Fprintf(&b, \"- %s(%s) %s%s [id:%s]\\n\", marca, topic, gist, age, memory.IDCorto(it.ID))\n"
// arnes: a="\t\t\tfmt.Fprintf(&b, \"- %s(%s) %s%s [id:%s]\\n\", marca, topic, gist, age, it.ID)\n"
// arnes: colision_ok="TestElPrimingMarcaIgualQueElTurno"
//
// Sabotaje: el priming sin topic emite el UUID entero en vez de la ref corta.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\t\tfmt.Fprintf(&b, \"- %s%s%s [id:%s]\\n\", marca, gist, age, memory.IDCorto(it.ID))\n"
// arnes: a="\t\t\tfmt.Fprintf(&b, \"- %s%s%s [id:%s]\\n\", marca, gist, age, it.ID)\n"
func TestElEmisorAcortaElIdEnLosCuatroSitios(t *testing.T) {
	const uuid = "0a1b2c3d-0000-4000-8000-000000000001"
	const corto = "0a1b2c3d" // los primeros 8 hex: lo que emite memory.IDCorto
	if memory.IDCorto(uuid) != corto {
		t.Fatalf("control: IDCorto(%q)=%q, esperaba %q — el UUID de prueba dejó de ser canónico y el "+
			"sabotaje de los cuatro sitios sería vacuo", uuid, memory.IDCorto(uuid), corto)
	}

	conTopic := memory.RecallItem{ID: uuid, TopicKey: "architecture/auth", Gist: "una nota cualquiera", CreatedAt: "2026-10-01"}
	sinTopic := memory.RecallItem{ID: uuid, Gist: "una nota cualquiera", CreatedAt: "2026-10-01"}

	casos := []struct {
		nombre string
		bloque string
	}{
		{"turno con topic", formatDeltaGists("[Musubi — memoria relevante] Contexto.", []memory.RecallItem{conTopic}, nil, "", nil)},
		{"turno sin topic", formatDeltaGists("[Musubi — memoria relevante] Contexto.", []memory.RecallItem{sinTopic}, nil, "", nil)},
		{"priming con topic", formatGists("[Musubi — memoria] Contexto.", memory.RecallResult{Count: 1, Items: []memory.RecallItem{conTopic}}, "")},
		{"priming sin topic", formatGists("[Musubi — memoria] Contexto.", memory.RecallResult{Count: 1, Items: []memory.RecallItem{sinTopic}}, "")},
	}
	for _, c := range casos {
		if !strings.Contains(c.bloque, "[id:"+corto+"]") {
			t.Errorf("%s: la viñeta no acortó el id a 8 hex; esperaba un [id:%s]\n%s", c.nombre, corto, c.bloque)
		}
		if strings.Contains(c.bloque, "[id:"+uuid+"]") {
			t.Errorf("%s: la viñeta emitió el UUID entero (36 chars), no la ref corta\n%s", c.nombre, c.bloque)
		}
	}
}
