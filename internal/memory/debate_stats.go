package memory

import "context"

// debate_stats.go es la lectura que necesita `musubi arnes` para contestar dos de sus cuatro
// preguntas: si los debates CIERRAN, y cuántas vueltas toma una corrección de verdad.
//
// Va como método del engine y NO se suma a la interfaz DebateStore a propósito: los comandos de
// cmd/ ya trabajan con *DbEngine concreto, y agregarlo a la interfaz obligaría a tocar backend.go
// —un archivo que otras ramas de este mismo track ya están editando— sin ganar nada.

// EstadoDeLosDebates es el resumen que se lee para saber si el arnés se está usando.
type EstadoDeLosDebates struct {
	Abiertos  int      `json:"abiertos"`
	Cerrados  int      `json:"cerrados"`
	Ganadores []string `json:"ganadores"`
	// Topics de los debates, para poder contar las cadenas de corrección: la convención de F4
	// mete «vuelta k/K» en el topic, que es texto libre.
	Topics []string `json:"topics"`
}

// EstadoDebatesCtx cuenta los debates por estado y devuelve sus topics y ganadores.
//
// Un debate ABIERTO para siempre no es un debate en curso: es un panel que alguien lanzó y nadie
// recontó. Por eso los dos números van juntos — el de cerrados solo no distingue «se usa poco» de
// «se abandona a la mitad».
func (e *DbEngine) EstadoDebatesCtx(ctx context.Context) (EstadoDeLosDebates, error) {
	var st EstadoDeLosDebates
	rows, err := e.db.QueryContext(ctx, `SELECT status, COALESCE(winner,''), COALESCE(topic,'') FROM debates`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var status, winner, topic string
		if err := rows.Scan(&status, &winner, &topic); err != nil {
			return st, err
		}
		if status == DebateClosed {
			st.Cerrados++
			if winner != "" {
				st.Ganadores = append(st.Ganadores, winner)
			}
		} else {
			st.Abiertos++
		}
		if topic != "" {
			st.Topics = append(st.Topics, topic)
		}
	}
	return st, rows.Err()
}
