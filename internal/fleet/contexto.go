package fleet

// contexto.go es la segunda mitad de la fase 5. La cronología (S13) contesta QUÉ HIZO Musubi en
// una máquina; esto contesta QUÉ SABÍA — y cruzarlas es lo único de todo el track que ningún panel
// del mercado puede dar, porque ningún panel tiene al lado la memoria del equipo y su código.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// CORRELACIÓN, NO CAUSA — Y ESO NO ES UNA ADVERTENCIA LEGAL, ES EL DISEÑO
//
// La tentación evidente es que la tool conteste «esta máquina anda lenta PORQUE el martes se
// desplegó X». No lo hace, y no por prudencia: lo haría **adivinando**, y una causa adivinada con
// aire de certeza es peor que no decir nada — manda a alguien a arreglar lo que no está roto y,
// la segunda vez que acierta, se le empieza a creer.
//
// Lo que hace es juntar los hechos y DECIR CÓMO LOS ENLAZÓ. Quien concluye es quien lee.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS TÉRMINOS NO SE ADIVINAN DEL TEXTO, SE TOMAN DEL INVENTARIO
//
// La otra tentación es sacar palabras del argv de los comandos: de `systemctl restart nginx`
// caería `nginx` y parecería listo. Sobre datos reales eso produce basura —`cmd`, `type`, `/c`,
// una ruta de Windows— y una lista de términos con basura adentro hace que la mitad de los
// enlaces sean ruido, que es la forma más rápida de que alguien deje de mirar la herramienta.
//
// Así que los términos salen de lo que Musubi YA SABE que existe en esa máquina: su nombre y los
// servicios declarados o reportados. Son pocos, son exactos y son explicables. Un término que no
// sirve es un problema de inventario, no de heurística — y ése sí se arregla.

import "strings"

const (
	// TerminoMinLargo descarta lo que no puede buscar bien. Un término de dos letras hace match
	// con media memoria y el enlace deja de significar nada.
	TerminoMinLargo = 3
	// TerminosMax acota cuántas búsquedas dispara una sola llamada. No es una regla de negocio:
	// es que cada término es una consulta FTS, y una máquina con sesenta servicios haría sesenta.
	TerminosMax = 12
	// ContextoTopeMemoria y ContextoTopeCodigo acotan lo que vuelve por cada eje.
	ContextoTopeMemoria = 20
	ContextoTopeCodigo  = 20
	// ContenidoMax recorta el texto de una observación. La memoria guarda notas largas y una
	// respuesta con veinte notas enteras no la lee nadie; para el texto completo está el recall.
	ContenidoMax = 400
)

// OrigenDeTermino dice DE DÓNDE salió un término, y viaja en la respuesta. Sin eso, quien lee no
// puede juzgar si el enlace vale: «apareció porque es el nombre de la máquina» y «apareció porque
// es un servicio que corre ahí» son dos niveles de evidencia distintos.
type OrigenDeTermino string

const (
	TerminoDeMaquina  OrigenDeTermino = "maquina"
	TerminoDeServicio OrigenDeTermino = "servicio"
)

type Termino struct {
	Texto string
	De    OrigenDeTermino
}

// sufijosDeUnidad son las extensiones de systemd que se sacan del nombre antes de buscar.
//
// NADIE ESCRIBE `nginx.service` EN UNA NOTA. Escribe «nginx». Buscar el nombre completo no
// encontraría nada y el resultado se leería como «no hay nada escrito sobre nginx», que es una
// conclusión falsa sacada de un detalle de systemd.
//
// Se sacan sólo estos cuatro, que son los que el inventario puede traer. No es una limpieza
// genérica de puntos: un servicio que se llame `api.altura` conserva su nombre entero, porque ahí
// el punto es parte del nombre y no un sufijo de unidad.
var sufijosDeUnidad = []string{".service", ".timer", ".socket", ".target"}

// TerminosDeContexto arma la lista de términos a buscar en la memoria.
//
// EL NOMBRE DE LA MÁQUINA VA PRIMERO Y SIEMPRE. Es el término más preciso que existe —lo eligió
// una persona, es único en el proyecto y no depende de que nadie haya declarado servicios—, así
// que si el tope recorta, recorta lo otro.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS DECLARADOS ANTES QUE LOS REPORTADOS, Y NO ES UNA PREFERENCIA ESTÉTICA
//
// Un host enumera decenas de units de systemd —`avahi-daemon`, `NetworkManager-wait-online`,
// `systemd-udevd`— y el tope se llena con las primeras que vengan. Medido en producción: la
// primera corrida contra `musubi-server` gastó las doce ranuras en units del sistema y dejó
// afuera `alturito20`, que es el ÚNICO servicio del que alguien escribió algo alguna vez.
//
// El criterio no es adivinar cuál importa: `Declarado` ya significa **una persona puso esto acá a
// mano**. Eso es exactamente la señal de que a alguien le importó, y viene del inventario, no de
// una heurística sobre el nombre.
//
// Las dos listas tienen que venir YA COMPUERTADAS por el llamador: un término es información
// sobre la máquina, y la lista de sus servicios exige `metrics`. Ver el comentario de la tool.
func TerminosDeContexto(device string, declarados, reportados []string) []Termino {
	out := make([]Termino, 0, TerminosMax)
	vistos := map[string]bool{}

	agregar := func(texto string, de OrigenDeTermino) {
		texto = strings.TrimSpace(texto)
		for _, sufijo := range sufijosDeUnidad {
			if strings.HasSuffix(strings.ToLower(texto), sufijo) {
				texto = texto[:len(texto)-len(sufijo)]
				break
			}
		}
		if len([]rune(texto)) < TerminoMinLargo || len(out) >= TerminosMax {
			return
		}
		// La deduplicación es insensible a mayúsculas porque la búsqueda también lo es: dejar
		// `Nginx` y `nginx` dispararía dos consultas idénticas y devolvería cada acierto dos
		// veces, con dos enlaces que dicen lo mismo.
		clave := strings.ToLower(texto)
		if vistos[clave] {
			return
		}
		vistos[clave] = true
		out = append(out, Termino{Texto: texto, De: de})
	}

	agregar(device, TerminoDeMaquina)
	for _, s := range declarados {
		agregar(s, TerminoDeServicio)
	}
	for _, s := range reportados {
		agregar(s, TerminoDeServicio)
	}
	return out
}

// Enlace es CÓMO se relacionó un hallazgo con esta máquina, y es el campo más importante de toda
// la respuesta.
//
// Mezclar los dos en una sola lista sería la mentira de este slice: «esta nota menciona nginx» y
// «esta nota se escribió la misma tarde» son evidencias de peso incomparable, y presentadas
// iguales convierten cualquier coincidencia temporal en una pista.
type Enlace string

const (
	// EnlacePorTermino: el texto NOMBRA algo de esta máquina. Es evidencia.
	EnlacePorTermino Enlace = "termino"
	// EnlacePorVentana: sólo coincide en el tiempo, en el mismo proyecto. NO es evidencia de
	// nada por sí solo; es contexto para que una persona mire.
	EnlacePorVentana Enlace = "ventana"
)

// HuecosDelContexto declara qué NO significa esta respuesta. Viaja siempre, igual que los huecos
// de la cronología, y por el mismo motivo: una lista de coincidencias sin sus límites al lado se
// lee como una explicación.
func HuecosDelContexto() []string {
	return []string{
		"Esto es CORRELACIÓN, no causa. Musubi junta lo que pasó y lo que estaba escrito en la misma ventana; NO afirma que una cosa haya causado la otra, y no tiene con qué afirmarlo.",
		"Un enlace `ventana` significa SÓLO que coincide en el tiempo y en el proyecto. No dice que la nota hable de esta máquina.",
		"Un enlace `termino` significa que el texto NOMBRA la máquina o uno de sus servicios. Puede ser homónimo: un `nginx` de otra máquina se escribe igual.",
		"La memoria sin proyecto asignado se ve desde CUALQUIER proyecto (es el criterio histórico del recall), así que una nota vieja sin atribuir puede aparecer acá.",
		"Los términos salen del INVENTARIO —el nombre de la máquina y sus servicios—, no del texto de los comandos. Una máquina sin servicios declarados enlaza sólo por su nombre.",
	}
}
