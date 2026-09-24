package main

import (
	"fmt"
	"io"
	"time"

	"musubi/internal/cerebro"
)

// esperaDelSondeo es cuánto aguanta `--alcance` antes de darse por vencido.
//
// NO ES EL `--timeout` DEL CANAL, y separarlo es el punto. Ese vale 60 s porque cubre a un central
// vivo pero LENTO contestando un `tools/list` de 53 tools. Éste contesta una pregunta distinta —
// «¿hay alguien ahí?»— que contra el cerebro sano se resuelve en ~200 ms (medido el 2026-09-22).
// Un diagnóstico que tarda un minuto en decir «no llegué» se deja de correr, y entonces no
// diagnostica nada.
const esperaDelSondeo = 8 * time.Second

// informarAlcance sondea el cerebro y escribe lo que midió. Devuelve el código de salida: 0 sólo
// si contestó 200, para que sirva adentro de un `&&` o de un guion de despliegue.
//
// LO QUE ESTE COMANDO REEMPLAZA es un literal escrito a mano en el manual de operación. Medido el
// 2026-09-22, ese literal devolvía `000` contra un cerebro sano —seguía nombrando el `7717`, que
// desde #601 es loopback y en claro— y el `000` se leía como «el cerebro está caído». Un literal
// envejece callado; un sondeo vuelve a preguntar.
//
// POR QUÉ IMPRIME LA DEMORA AUNQUE HAYA FALLADO: separa los fallos que un código de estado no
// distingue. Un corte de handshake vuelve en milisegundos y un destino que no contesta agota la
// espera entera; con el número a la vista, «no llegué en 0,1 s» y «no llegué en 8 s» dejan de ser
// la misma línea.
func informarAlcance(w io.Writer, base, nombre string, espera time.Duration) int {
	a := cerebro.SondearCon(base, nombre, espera, nil)

	fmt.Fprintf(w, "cerebro   %s%s\n", a.Base, cerebro.RutaDeSalud)
	if nombre != "" {
		fmt.Fprintf(w, "nombre    %s  (%s)\n", nombre, cerebro.EnvNombreTLS)
	}

	switch {
	case a.Err != nil:
		fmt.Fprintf(w, "resultado NO CONTESTÓ en %v\n", a.Demora.Round(time.Millisecond))
		fmt.Fprintf(w, "error     %v\n", a.Err)
	case a.Llego():
		fmt.Fprintf(w, "resultado %d en %v   %s\n", a.Codigo, a.Demora.Round(time.Millisecond), a.Cuerpo)
	default:
		fmt.Fprintf(w, "resultado %d en %v   %s\n", a.Codigo, a.Demora.Round(time.Millisecond), a.Cuerpo)
		fmt.Fprintf(w, "          contestó, pero no con 200: hay algo escuchando ahí y no está listo\n")
	}

	// LA CAUSA SE IMPRIME SÓLO SI SE LA PUDO DECIDIR. Una línea `causa` vacía o con una conjetura
	// es peor que ninguna: se la cree. Ver `causaDecidible`.
	if a.Causa != "" {
		fmt.Fprintf(w, "causa     %s\n", a.Causa)
	}

	if a.Llego() {
		return 0
	}
	return 1
}
