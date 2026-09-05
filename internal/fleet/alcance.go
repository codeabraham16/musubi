package fleet

// alcance.go responde una pregunta que NINGUNA otra parte de este track puede responder:
// **desde ESTA máquina, ¿se llega a ese puerto?** (A67)
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ HACE FALTA, Y POR QUÉ EL CHEQUEO QUE YA EXISTÍA NO ALCANZABA
//
// El colector del relay sondea sus tres puertos DESDE EL PROPIO SERVIDOR. Eso verifica que el
// servicio contesta ahí, que es lo único que ese punto de vista puede ver — y no es la pregunta.
// La pregunta de un relay es si un CLIENTE puede alcanzarlo.
//
// Medido el 2026-09-01, con el relay «sano» por las tres vías que existían (dos contenedores
// arriba y los tres puertos contestando en loopback): una máquina daba `False` y la otra `True`
// contra el MISMO puerto. La salud era distinta según quién preguntara, y el chequeo sólo conocía
// la respuesta del que nunca falla. La causa concreta fue el VPN de una de las dos, pero el
// agujero es independiente de la causa: **un punto de vista único no puede medir alcanzabilidad.**
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// AUSENTE NO ES FALSO, Y ACÁ LA DIFERENCIA ES CARA
//
// Una máquina que no tiene destinos configurados no reporta sondas. Eso NO significa «no llega»:
// significa «nadie le pidió que mirara». Si se emitiera un 0, cada máquina sin configurar
// dispararía la alerta del relay y en dos días nadie la mira — que es la forma exacta en que una
// alarma se vuelve ruido. Es la misma regla que gobierna el track entero desde S4: un dato que no
// se pudo medir se OMITE, no se inventa un cero.

import (
	"strconv"
	"strings"
	"time"
)

const (
	// AlcanceMaxDestinos acota cuántos puertos sondea una máquina por latido. El latido es de
	// segundos y cada sonda es una conexión TCP que puede colgarse hasta el timeout: sin techo,
	// una lista larga convierte el latido en un escáner de puertos y lo atrasa.
	AlcanceMaxDestinos = 4

	// AlcanceTimeout es cuánto espera UNA sonda. Corto a propósito: la pregunta no es «cuánto
	// tarda» sino «llega o no», y un destino que tarda tres segundos en aceptar una conexión ya
	// es un problema aunque termine aceptándola.
	AlcanceTimeout = 3 * time.Second
)

// SondaDeAlcance es el resultado de UNA sonda desde UNA máquina.
//
// No lleva latencia a propósito: agregarla invitaría a alertar por «lento», y un TCP connect
// contra un relay no mide nada parecido a la calidad de una sesión remota. Medir de más es cómo
// se termina alertando sobre lo que no importa.
type SondaDeAlcance struct {
	// Destino es `host:puerto`, tal como lo escribió quien configuró la máquina.
	Destino string `json:"destino"`
	// Alcanza es la respuesta. Una sonda que NO SE PUDO CORRER no produce una entrada con
	// `false`: no produce entrada. Ver el encabezado.
	Alcanza bool `json:"alcanza"`
}

// DestinoDeAlcanceValido acepta `host:puerto` con un puerto en rango.
//
// FAIL-CLOSED: un destino mal escrito se descarta al configurar y no llega nunca a sondearse. La
// alternativa —sondearlo igual y reportar `false`— haría que un typo se vea idéntico a un relay
// caído, y son dos problemas de dos personas distintas.
func DestinoDeAlcanceValido(d string) bool {
	d = strings.TrimSpace(d)
	if d == "" || strings.ContainsAny(d, " \t\r\n") {
		return false
	}
	i := strings.LastIndex(d, ":")
	if i <= 0 || i == len(d)-1 {
		return false
	}
	host := d[:i]
	if host == "" || strings.Contains(host, ",") {
		return false
	}
	p, err := strconv.Atoi(d[i+1:])
	return err == nil && p >= 1 && p <= 65535
}

// LimpiarDestinosDeAlcance normaliza la lista configurada: descarta los inválidos, saca repetidos
// y aplica el tope. Devuelve también cuántos quedaron afuera, para que quien configuró se entere
// en vez de descubrirlo por una serie que no aparece.
func LimpiarDestinosDeAlcance(in []string) (destinos []string, descartados int) {
	vistos := make(map[string]bool, len(in))
	for _, d := range in {
		d = strings.TrimSpace(d)
		if !DestinoDeAlcanceValido(d) || vistos[d] {
			if d != "" {
				descartados++
			}
			continue
		}
		if len(destinos) >= AlcanceMaxDestinos {
			descartados++
			continue
		}
		vistos[d] = true
		destinos = append(destinos, d)
	}
	return destinos, descartados
}
