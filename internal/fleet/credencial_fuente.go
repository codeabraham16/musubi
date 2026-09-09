package fleet

// credencial_fuente.go — DE DÓNDE SALIÓ EL TOKEN DEL DISPOSITIVO, dicho por el agente.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EL CEREBRO NECESITA SABERLO (A102)
//
// El agente acepta su credencial por DOS caminos y no son equivalentes: `MUSUBI_DEVICE_TOKEN_FILE`
// —una ruta, que el agente puede REESCRIBIR cuando el cerebro le ofrece un token nuevo en la
// respuesta del latido— y `MUSUBI_DEVICE_TOKEN` —el valor en el entorno, que un proceso no puede
// cambiarse a sí mismo—. La ayuda del propio binario ya lo dice: el archivo «es el único de los dos
// con el que se puede ROTAR la credencial en caliente».
//
// Así que una máquina que recibió su token por variable NO PUEDE COMPLETAR UNA ROTACIÓN, y eso es
// invisible desde el cerebro: late igual, reporta igual, y la rotación vence siempre. Medido el
// 2026-09-05 en `davantis-1`, donde el lanzador tenía la forma vieja —`set /p MUSUBI_DEVICE_TOKEN=<
// archivo`— y el arreglo estaba en el repo desde antes sin haber llegado a la máquina.
//
// EL COSTO DE NO SABERLO SE PAGÓ DOS VECES: la rotación que «no podía completarse» quedó anotada
// como síntoma sin causa, y para encontrarla hubo que LEER UN .cmd EN LA MÁQUINA. Con esto, la
// pregunta «¿qué máquinas no pueden rotar?» se le hace al cerebro.
//
// ES UN HECHO MEDIDO Y NO UNA CONFIGURACIÓN, igual que `puede_preguntar`: lo reporta quien lo sabe
// —el proceso que abrió la credencial— y cambia con el mundo, no con un archivo de política.
//
// LOS VALORES SON TEXTO Y NO UN BOOLEANO, y el vacío es un tercer estado con significado. Un
// booleano no puede decir «este agente no opinó»: un agente viejo no manda el campo, y leerlo como
// «no puede rotar» sería ACUSAR a toda la flota desplegada de un defecto que nadie midió. Es el
// mismo criterio con el que `consentimiento` arrancó vacío en vez de con un grado.
const (
	// CredencialDeArchivo: vino por `MUSUBI_DEVICE_TOKEN_FILE`. Una rotación se puede adoptar.
	CredencialDeArchivo = "archivo"
	// CredencialDeVariable: vino por `MUSUBI_DEVICE_TOKEN`. Una rotación NO se puede adoptar, y el
	// token además queda en el entorno del proceso —donde lo lee cualquier proceso del mismo
	// usuario, y donde sobrevive a que alguien arregle el archivo—.
	CredencialDeVariable = "variable"
)

// CredencialRotable dice si con esa fuente se puede completar una rotación.
//
// Devuelve (rotable, seSabe). El segundo es el que evita el peor error de lectura: sin él, «no
// rotable» y «no lo dijo» se confunden, y el exportador publicaría un 0 para una máquina que nunca
// opinó. Es la misma distinción que este plano mantiene en todas las series: AUSENTE NO ES CERO.
func CredencialRotable(fuente string) (rotable, seSabe bool) {
	switch fuente {
	case CredencialDeArchivo:
		return true, true
	case CredencialDeVariable:
		return false, true
	default:
		return false, false
	}
}

// FuenteDeCredencialValida acota lo que el cerebro acepta guardar. El vacío ES válido: significa
// «no lo dijo». Un valor desconocido se rechaza en vez de guardarse, porque una fila con un texto
// que el dominio no entiende se lee después como si significara algo.
func FuenteDeCredencialValida(fuente string) bool {
	switch fuente {
	case "", CredencialDeArchivo, CredencialDeVariable:
		return true
	}
	return false
}
