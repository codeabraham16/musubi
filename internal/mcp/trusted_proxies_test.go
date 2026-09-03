package mcp

import (
	"net/http"
	"testing"
)

// EL LOCKOUT MIRA LA IP DEL CLIENTE, Y `X-Forwarded-For` SÓLO SE CREE DESDE UN ORIGEN DECLARADO.
//
// Ola 1 del plan empresa. Dos modos de falla opuestos, y las dos mitades tienen que valer a la vez:
//
//	· Sin `trusted_proxies`, leer el header convertiría el lockout en decorativo: quien prueba
//	  tokens manda una IP inventada distinta en cada intento y no se bloquea nunca.
//	· Detrás de un proxy o un VIP —que es lo que trae la HA de la Ola 5— TODOS los agentes llegan
//	  con la IP del proxy, y cinco tokens malos de UNA máquina bloquean a la célula entera.
//
// Sabotaje que la hace fallar: leer el header sin mirar `ipConfiable` (rompe el primer caso), o
// devolver siempre `RemoteAddr` (rompe el segundo).
func TestElHeaderDeProxySoloSeCreeDesdeUnOrigenDeclarado(t *testing.T) {
	anterior := proxiesConfiables
	t.Cleanup(func() { proxiesConfiables = anterior })

	pedido := func(remoto, xff string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remoto
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}

	// SIN configurar: el header se ignora, pase lo que pase. Es el default y el de siempre.
	proxiesConfiables = nil
	if got := clientIP(pedido("203.0.113.9:5555", "1.2.3.4")); got != "203.0.113.9" {
		t.Errorf("sin trusted_proxies se creyó el header: %q. El lockout queda decorativo — una IP inventada por intento y nunca se bloquea", got)
	}

	if err := fijarProxiesConfiables([]string{"10.0.0.0/8"}); err != nil {
		t.Fatal(err)
	}

	// Desde un proxy CONFIABLE: se cree el header.
	if got := clientIP(pedido("10.1.2.3:443", "198.51.100.7")); got != "198.51.100.7" {
		t.Errorf("detrás del proxy confiable el cliente salió %q: todos los agentes comparten IP y cinco tokens malos bloquean a la célula entera", got)
	}
	// Desde una IP NO confiable: el header se ignora aunque venga.
	if got := clientIP(pedido("203.0.113.9:5555", "198.51.100.7")); got != "203.0.113.9" {
		t.Errorf("se creyó el header desde un origen no declarado: %q", got)
	}
	// EL CASO QUE IMPORTA, Y ES EL ATAQUE: el cliente escribe su propio `X-Forwarded-For` para
	// escapar del lockout. Nuestro proxy NO lo borra, lo APENDEA — así que la cadena queda
	// `<mentira del cliente>, <lo que el proxy vio de verdad>, <saltos internos>`.
	//
	// De derecha a izquierda saltando proxies nuestros, gana 198.51.100.7, que es el último salto
	// que un proxy nuestro vio con sus propios ojos. De izquierda a derecha gana 1.2.3.4, que la
	// escribió el atacante — y como puede poner una distinta en cada intento, nunca se bloquea.
	//
	// Sin este caso la prueba no distingue las dos direcciones: con la cadena sin mentira las dos
	// dan la misma respuesta, y el sabotaje pasaba en verde.
	if got := clientIP(pedido("10.0.0.1:443", "1.2.3.4, 198.51.100.7, 10.0.0.2")); got != "198.51.100.7" {
		t.Errorf("con un `X-Forwarded-For` falsificado por el cliente salió %q, esperaba 198.51.100.7: recorrer de izquierda a derecha toma la mentira del atacante, que puede poner una IP distinta en cada intento y no bloquearse nunca", got)
	}
	// Y la cadena honesta sigue dando lo mismo.
	if got := clientIP(pedido("10.0.0.1:443", "198.51.100.7, 10.9.9.9, 10.0.0.2")); got != "198.51.100.7" {
		t.Errorf("con varios saltos honestos el cliente salió %q, esperaba 198.51.100.7", got)
	}
	// Basura en el header: se ignora, no se confía.
	if got := clientIP(pedido("10.0.0.1:443", "no-es-una-ip")); got != "10.0.0.1" {
		t.Errorf("una IP inválida en el header se tomó como cliente: %q", got)
	}
	// Todos los saltos son proxies nuestros: queda la directa, que es la verdad aunque no sirva
	// para distinguir clientes.
	if got := clientIP(pedido("10.0.0.1:443", "10.9.9.9, 10.0.0.2")); got != "10.0.0.1" {
		t.Errorf("con la cadena entera de proxies confiables salió %q, esperaba la directa", got)
	}
}

// UN CIDR ILEGIBLE IMPIDE ARRANCAR, y no deja una lista a medias.
//
// Una lista a medias deja el lockout mirando la IP equivocada sin que nadie se entere: el modo de
// falla de un typo tiene que ser ruidoso, igual que en `expires:` y en el resto del arranque.
//
// Sabotaje: que fijarProxiesConfiables saltee el CIDR malo en vez de devolver error.
func TestUnCidrIlegibleImpideArrancar(t *testing.T) {
	anterior := proxiesConfiables
	t.Cleanup(func() { proxiesConfiables = anterior })

	err := fijarProxiesConfiables([]string{"10.0.0.0/8", "esto-no-es-un-cidr"})
	if err == nil {
		t.Fatal("un CIDR ilegible se aceptó: el lockout queda mirando la IP equivocada y nadie se entera")
	}
	// Y no dejó media lista puesta.
	if len(proxiesConfiables) != 0 && anterior == nil {
		t.Errorf("quedó una lista a medias con %d redes: un arranque que falla no puede dejar estado aplicado", len(proxiesConfiables))
	}
}
