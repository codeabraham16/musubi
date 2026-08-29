package fleet

// rendimiento.go agrega la medida que a Musubi le faltaba: qué HIZO un servicio, no en qué estado
// está.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL HUECO QUE CIERRA
//
// `musubi_fleet_service_declare` existe —lo dice su propia descripción— para declarar «un Tier B
// que no enumera solo, un bot, un puente». Y hasta acá un bot declarado se quedaba en
// `desconocido` PARA SIEMPRE: su salud «pasa a tener estado cuando la máquina lo reporte», y a un
// bot que vive en una base gestionada en la nube no lo reporta ninguna máquina. La capacidad
// estaba declarada y era inalcanzable.
//
// Lo que un bot tiene para decir tampoco entra en `SaludServicio`: no es «corriendo» ni «fallado»,
// es «atendí 47 consultas en el último minuto, 3 salieron mal, el p95 fue de 820 ms».
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL CERO ACÁ ES UN DATO, Y ES EL DATO MÁS IMPORTANTE
//
// Es al revés que en todo el resto del track. En una muestra de host, un 0 sospechoso significa
// «no medido» y por eso viaja como null. Acá **`Atendidas == 0` es una MEDICIÓN**: quiere decir
// «miré y no pasó nada», que es exactamente lo que distingue un bot callado de un colector muerto.
//
// La distinción se sostiene un nivel más arriba y no dentro de la struct: `Rendimiento == nil` es
// «no se midió», y un `Rendimiento` presente con todo en cero es «se midió, no hubo nada». Por eso
// los conteos son `int` y no punteros, y las latencias SÍ son punteros: un p95 de 0 ms sobre cero
// consultas no es un percentil, es la ausencia de uno.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA VENTANA VIAJA CON LOS NÚMEROS
//
// «47 atendidas» no significa nada sin saber en cuánto tiempo. Guardar el conteo sin su ventana
// obliga a deducirla del intervalo del reportante, que es un número que vive en otro archivo y que
// alguien cambia por otra razón — el mismo error que scheduler_flota.go documenta al negarse a
// colgar el empuje del intervalo de sondeo.

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// DesgloseMax acota cuántos resultados distintos se aceptan. Las claves las elige quien
	// reporta (ENTRADA NO CONFIABLE: son nombres de su dominio, no de éste) y terminan dibujadas
	// en el panel y potencialmente como etiquetas en Prometheus, donde la cardinalidad se paga.
	DesgloseMax = 12
	// DesgloseClaveMax acota el largo de cada clave, por lo mismo.
	DesgloseClaveMax = 32
	// VentanaMaxSeg es el techo de la ventana: un día. Más que eso no es un reporte periódico,
	// es un backfill, y un backfill que entra por el camino del latido descoloca cualquier tasa.
	VentanaMaxSeg = 86400
)

// Rendimiento es lo que un servicio hizo en una ventana de tiempo.
type Rendimiento struct {
	// VentanaSeg es cuánto tiempo cubren estos números. Obligatorio y > 0.
	VentanaSeg int `json:"ventana_seg"`
	// Atendidas es cuántas unidades de trabajo manejó. CERO ES UNA MEDICIÓN (ver el encabezado).
	Atendidas int `json:"atendidas"`
	// Fallidas es cuántas salieron mal. Es un subconjunto de Atendidas, nunca un total aparte.
	Fallidas int `json:"fallidas"`
	// Desglose son los resultados con los NOMBRES DEL DOMINIO de quien reporta (`ok`, `no_puedo`,
	// `vacio`…). Musubi no los interpreta: los guarda y los muestra. Interpretarlos sería
	// meterle el vocabulario de un cliente al plano de control.
	//
	// PUEDE SUMAR MENOS QUE Atendidas y está bien: quien reporta puede no saber clasificar todo,
	// y forzar que cierre lo empujaría a inventar una categoría «otros» que no midió.
	Desglose map[string]int `json:"desglose,omitempty"`
	// LatenciaP95Ms y LatenciaMaxMs son punteros: nil = no se midió. Un 0 sobre cero unidades no
	// es un percentil bajo, es la ausencia de percentil.
	LatenciaP95Ms *int `json:"latencia_p95_ms,omitempty"`
	LatenciaMaxMs *int `json:"latencia_max_ms,omitempty"`
}

// Valida rechaza un rendimiento que no se puede creer. ENTRADA NO CONFIABLE: lo manda un colector
// que Musubi no controla, igual que la muestra de un agente, y se rechaza ENTERO en vez de
// «corregirlo» — la misma asimetría D7 que usa Muestra.Valida.
func (r *Rendimiento) Valida() error {
	if r == nil {
		return nil // no medido: es un estado legítimo, no una falla
	}
	if r.VentanaSeg <= 0 {
		return fmt.Errorf("ventana_seg es %d: un conteo sin su ventana no se puede leer como tasa", r.VentanaSeg)
	}
	if r.VentanaSeg > VentanaMaxSeg {
		return fmt.Errorf("ventana_seg es %d, más de un día: eso es un backfill, y un backfill "+
			"entrando por el camino del reporte periódico descoloca cualquier tasa", r.VentanaSeg)
	}
	if r.Atendidas < 0 {
		return fmt.Errorf("atendidas es negativo: %d", r.Atendidas)
	}
	if r.Fallidas < 0 {
		return fmt.Errorf("fallidas es negativo: %d", r.Fallidas)
	}
	// FALLIDAS ES UN SUBCONJUNTO, NO UN TOTAL APARTE. Sin esta regla, «3 atendidas y 7 fallidas»
	// pasa y después una tasa de error da 233 %, que se lee como un bug del panel y no como lo
	// que es: un reportante que cuenta dos cosas distintas con los mismos nombres.
	if r.Fallidas > r.Atendidas {
		return fmt.Errorf("fallidas (%d) supera atendidas (%d): las fallidas son un subconjunto, "+
			"no un total aparte", r.Fallidas, r.Atendidas)
	}
	if n := len(r.Desglose); n > DesgloseMax {
		return fmt.Errorf("el desglose trae %d resultados distintos y el tope es %d: las claves "+
			"las elige quien reporta y terminan como etiquetas, donde la cardinalidad se paga", n, DesgloseMax)
	}
	total := 0
	for clave, v := range r.Desglose {
		if strings.TrimSpace(clave) == "" {
			return fmt.Errorf("el desglose trae una clave vacía")
		}
		if len([]rune(clave)) > DesgloseClaveMax {
			return fmt.Errorf("la clave %q del desglose supera %d caracteres", clave, DesgloseClaveMax)
		}
		if v < 0 {
			return fmt.Errorf("el desglose trae %q en %d, negativo", clave, v)
		}
		total += v
	}
	// SUMAR MENOS ESTÁ BIEN, SUMAR MÁS NO. Menos significa «no supe clasificar todo», que es
	// honesto. Más significa que el desglose y el total cuentan cosas distintas, y entonces
	// ninguno de los dos se puede usar.
	if total > r.Atendidas {
		return fmt.Errorf("el desglose suma %d y atendidas es %d: el desglose y el total están "+
			"contando cosas distintas", total, r.Atendidas)
	}
	if err := validaLatencia("latencia_p95_ms", r.LatenciaP95Ms); err != nil {
		return err
	}
	if err := validaLatencia("latencia_max_ms", r.LatenciaMaxMs); err != nil {
		return err
	}
	if r.LatenciaP95Ms != nil && r.LatenciaMaxMs != nil && *r.LatenciaP95Ms > *r.LatenciaMaxMs {
		return fmt.Errorf("el p95 (%d ms) supera el máximo (%d ms)", *r.LatenciaP95Ms, *r.LatenciaMaxMs)
	}
	// UNA LATENCIA SIN NADA QUE MEDIR NO ES CERO, ES NADA. Un colector que manda p95=0 con
	// atendidas=0 no midió un percentil bajo: no midió. Dejarlo pasar mete un 0 en la serie que
	// tira cualquier promedio hacia abajo justo en los minutos tranquilos.
	if r.Atendidas == 0 && (r.LatenciaP95Ms != nil || r.LatenciaMaxMs != nil) {
		return fmt.Errorf("vino latencia con atendidas=0: sin unidades no hay percentil, y un 0 " +
			"ahí hunde el promedio justo en los minutos tranquilos")
	}
	return nil
}

func validaLatencia(nombre string, v *int) error {
	if v != nil && *v < 0 {
		return fmt.Errorf("%s es negativo: %d", nombre, *v)
	}
	return nil
}

// TasaDeError es fallidas sobre atendidas, en porcentaje. El bool es false cuando NO HAY TASA:
// cero atendidas no es 0 % de error, es la ausencia de una tasa — y un 0 % pintado sobre un
// servicio que no atendió nada se lee como «todo perfecto».
func (r *Rendimiento) TasaDeError() (float64, bool) {
	if r == nil || r.Atendidas <= 0 {
		return 0, false
	}
	return 100 * float64(r.Fallidas) / float64(r.Atendidas), true
}

// PorSegundo es la tasa de trabajo. El bool es false sin ventana: un conteo sin su ventana no es
// una tasa, y Valida ya lo rechaza — esto es la segunda línea, para el que lea de la base una fila
// vieja escrita antes de la regla.
func (r *Rendimiento) PorSegundo() (float64, bool) {
	if r == nil || r.VentanaSeg <= 0 {
		return 0, false
	}
	return float64(r.Atendidas) / float64(r.VentanaSeg), true
}

// ClavesDelDesglose devuelve los resultados ORDENADOS. El orden de un map de Go es aleatorio por
// diseño, y un panel que reordena sus columnas en cada refresco es ilegible.
func (r *Rendimiento) ClavesDelDesglose() []string {
	if r == nil || len(r.Desglose) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.Desglose))
	for k := range r.Desglose {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
