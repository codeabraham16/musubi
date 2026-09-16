package mcp

import (
	"testing"
	"time"
)

// UNA ROTACIÓN VENCIDA NO VALE AUNQUE NADIE HAYA BARRIDO TODAVÍA.
//
// `rotacion_vence` existe para acotar cuánto vive un token que se EMITIÓ y que el agente todavía
// no confirmó. Pero quien decidía si ese token servía no era la fecha: era
// `AbandonarRotacionesVencidas`, un UPDATE que corre al final del barrido de flota. La consulta
// que resuelve el token nuevo preguntaba sólo por la columna del token nuevo no vacia y `revoked = 0`.
//
// O sea que la vigencia dependía de que alguien hubiera pasado a limpiar, y ese barrido tiene
// salidas tempranas que no tienen nada que ver con las rotaciones: un barrido anterior en vuelo,
// una lista de proyectos ilegible, el contexto cancelado. Mientras cualquiera de esas dure, un
// token vencido sigue autenticando — y el latido no sólo lo acepta: COMPLETA la rotación con él,
// así que el token que debía morir se vuelve el definitivo.
//
// ES EL MISMO CRITERIO QUE ESTE REPO YA APLICÓ EN LAS SESIONES DE SHELL: ahí `Vencida` se DERIVA
// y no se guarda, con el argumento de que «una columna de estado que alguien tiene que ir a
// actualizar miente en cuanto nadie la actualiza». Acá la columna estaba, la fecha estaba, y la
// decisión se tomaba sin mirarla.
//
// LA PRUEBA QUE YA EXISTÍA NO CUBRE ESTO: `TestUnaRotacionVencidaSeAbandonaYElTokenViejoSigueValiendo`
// llama a `AbandonarRotacionesVencidas` a mano antes de preguntar. Mide el mundo en que el barrido
// SÍ corrió, que es el camino sano.
//
// Sabotaje: sacarle a la consulta del token nuevo el filtro por `rotacion_vence`, que es el
// estado anterior — la vigencia volvía a depender del UPDATE que pasa a limpiar.
//
// arnes: archivo="internal/memory/rotacion.go"
// EL CORTE EVITA LAS COMILLAS SIMPLES, A PROPÓSITO. Dos comillas simples seguidas dentro de un
// comentario de doc las convierte gofmt en una comilla tipográfica, y una directiva con la
// comilla cambiada apunta a un literal que no existe: el censo valida el `de`, así que el que se
// rompe en silencio es el `a`. Se corta por la fecha y su parámetro, que es lo que el sabotaje
// tiene que llevarse.
//
// arnes: de="AND rotacion_vence > ? AND revoked = 0`, h, ahora.UTC().Format(time.RFC3339))"
// arnes: a="AND revoked = 0`, h)"
// arnes: arreglo_de="AND rotacion_vence > ? AND revoked = 0"
// arnes: arreglo_a="AND revoked = 0 AND rotacion_vence > ?"
func TestElTokenDeUnaRotacionVencidaNoAutenticaAunqueNadieHayaBarrido(t *testing.T) {
	// CONTROL, PRIMERO — una rotación VIGENTE sí autentica con el token nuevo. Sin esto, la
	// guarda de abajo la satisface hacer que el token nuevo no valga nunca, que es romper la
	// rotación en caliente entera.
	t.Run("control: una rotación vigente autentica", func(t *testing.T) {
		s, ts, _, _ := servidorConFlota(t)
		d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
		tokenNuevo, err := s.engine.AbrirRotacion(d.ID, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tokenNuevo, `{"version":"0.1.0"}`); code != 200 {
			t.Fatalf("el token nuevo de una rotación VIGENTE no autentica (%d %s): la rotación en caliente dejaría al agente afuera", code, b)
		}
	})

	// EL CASO — la rotación venció y NADIE pasó a abandonarla.
	t.Run("vencida y sin barrido", func(t *testing.T) {
		s, ts, tokenViejo, _ := servidorConFlota(t)
		d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
		tokenNuevo, err := s.engine.AbrirRotacion(d.ID, time.Now().Add(-time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		// A PROPÓSITO NO SE LLAMA A AbandonarRotacionesVencidas: eso es lo que mide esta prueba.

		code, cuerpo := postCon(t, ts.URL+fleetHeartbeatPath, tokenNuevo, `{"version":"0.1.0"}`)
		if code != 401 {
			t.Errorf("el token de una rotación VENCIDA autenticó (%d %s). La fecha estaba en la fila y "+
				"la consulta no la miraba: quien decidía era el barrido que pasa a limpiar, y ese "+
				"barrido tiene tres salidas tempranas ajenas a las rotaciones.", code, cuerpo)
		}

		// Y LA MITAD QUE MÁS DUELE: si ese latido se acepta, además COMPLETA la rotación, o sea
		// que el token que tenía que morir se vuelve el definitivo y el viejo se borra.
		if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenViejo, `{"version":"0.1.0"}`); code != 200 {
			t.Error("el token VIEJO dejó de valer: el latido con un token vencido completó la rotación. " +
				"Una rotación que venció sin confirmarse tiene que abandonarse, no cerrarse sola.")
		}

		// El estado en la base tampoco puede haber cambiado: la rotación sigue abierta, para que
		// el barrido la abandone cuando pase.
		abiertas, err := s.engine.RotacionesAbiertas()
		if err != nil {
			t.Fatalf("no se pudieron listar las rotaciones abiertas: %v", err)
		}
		if len(abiertas) != 1 {
			t.Errorf("hay %d rotaciones abiertas y tenía que quedar 1: el latido con el token vencido la cerró", len(abiertas))
		}
	})
}
