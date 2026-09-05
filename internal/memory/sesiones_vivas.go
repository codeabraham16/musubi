package memory

// sesiones_vivas.go es la lectura ÚNICA del plano de entrar: quién está adentro de las máquinas
// de un proyecto, por cualquier modalidad, en una sola lista.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LEE DOS TABLAS Y NO LAS FUSIONA, Y ESO ES UNA DECISIÓN
//
// `screen_sessions` y `shell_sessions` tienen casi las mismas columnas, así que fusionarlas
// parece obvio. No lo es: sus comportamientos difieren donde importa —la shell tiene techo de
// inactividad, una sola sesión abierta por (principal × máquina) y un barrendero; la pantalla no
// tiene ninguna de las tres—. Una tabla común tendría columnas que sólo aplican a la mitad de sus
// filas, que es el olor de esquema que este repo evita en todos lados.
//
// Lo que se comparte es la FORMA DE AUDITORÍA, y eso es una vista. El razonamiento largo está en
// internal/fleet/sesion_viva.go.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL TOPE SE APLICA POR MODALIDAD Y DESPUÉS AL TOTAL, Y NO ES REDUNDANTE
//
// Sin el tope por modalidad, un proyecto con miles de sesiones de shell y tres de pantalla
// devolvería las tres de pantalla fuera del corte: el ORDER BY global las dejaría afuera aunque
// sean lo más reciente de su clase. El resultado se vería como «acá no hay sesiones de pantalla»,
// que es distinto de «hay, y no entraron».

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// SesionesVivas devuelve las sesiones de un proyecto por TODAS las modalidades, más nuevas
// primero. `deviceID` vacío = todas las máquinas.
//
// Los nombres de máquina se resuelven acá y no en el llamador: una lista de sesiones con ids
// opacos no la lee nadie, y hacer que cada consumidor los resuelva por su cuenta garantiza que
// uno se olvide.
func (e *DbEngine) SesionesVivas(projectID, deviceID string, tope int, ahora time.Time) ([]fleet.SesionViva, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || tope <= 0 {
		return nil, nil
	}

	// El nombre de cada máquina, una sola consulta. Se piden CON revocadas: una sesión abierta
	// sobre una máquina que después se dio de baja sigue siendo un hecho de la auditoría, y
	// perder su nombre justo ahí es perderlo cuando más se necesita.
	devices, err := e.ListarDevices(projectID, true)
	if err != nil {
		return nil, err
	}
	nombre := make(map[string]string, len(devices))
	for _, d := range devices {
		nombre[d.ID] = d.Name
	}

	var out []fleet.SesionViva

	pantallas, err := e.SesionesDePantalla(projectID, deviceID, tope, ahora)
	if err != nil {
		return nil, fmt.Errorf("error al leer las sesiones de pantalla: %w", err)
	}
	for _, s := range pantallas {
		out = append(out, fleet.DesdeSesionPantalla(s, nombre[s.DeviceID]))
	}

	shells, err := e.BitacoraDeShell(projectID, deviceID, tope)
	if err != nil {
		return nil, fmt.Errorf("error al leer las sesiones de shell: %w", err)
	}
	for _, s := range shells {
		out = append(out, fleet.DesdeSesionShell(s, nombre[s.DeviceID]))
	}

	// ORDEN ESTABLE, Y EL DESEMPATE NO ES ADORNO: dos sesiones creadas en el mismo instante —pasa
	// cuando alguien abre pantalla y shell juntas desde un panel— saldrían en orden distinto en
	// cada llamada, y una lista que se reordena sola mientras se mira es una lista en la que
	// nadie confía. El id desempata porque es lo único único que hay.
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Creada.Equal(out[j].Creada) {
			return out[i].Creada.After(out[j].Creada)
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > tope {
		out = out[:tope]
	}
	return out, nil
}
