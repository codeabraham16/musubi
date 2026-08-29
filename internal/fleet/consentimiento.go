package fleet

// consentimiento.go es QUÉ PASA EN LA MÁQUINA cuando alguien pide entrar, y es un eje SEPARADO
// de los permisos.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ES UN EJE APARTE Y NO UNA CAPACIDAD MÁS
//
// «Quién puede entrar» y «qué se hace con la persona que está adentro de la máquina» son dos
// preguntas distintas y las contestan dos personas distintas. La primera la contesta quien
// administra la flota, en `principals.yaml`. La segunda la contesta quien es DUEÑO de esa
// máquina: en la propia, nadie tiene que avisarle nada; en la de un cliente, mirar la pantalla
// sin permiso es —según dónde— un problema legal antes que técnico.
//
// Mezclarlas en una sola concesión obliga a elegir entre «puede entrar sin avisar» y «no puede
// entrar», que son las dos respuestas equivocadas para el caso más común: puede entrar, y le
// pregunta.
//
// Hasta acá Musubi no tenía NADA de esto. Una sesión de pantalla se abría y la persona sentada
// enfrente no se enteraba. Es la ausencia más grave del modelo, y la encontró comparar con
// MeshCentral, que tiene el eje entero (`USERCONSENT_DesktopNotifyUser`,
// `USERCONSENT_DesktopPromptUser`, y los gemelos para terminal y archivos).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS CUATRO GRADOS ESTÁN ORDENADOS, Y EL ORDEN ES LA REGLA
//
// No son etiquetas sueltas: son una escala de restricción creciente, y de ahí sale la única
// operación que importa —cuando dos fuentes dicen cosas distintas, GANA LA MÁS RESTRICTIVA—.
// Es la misma acumulación que hace MeshCentral con un OR de bits; acá es un máximo sobre la
// escala, que dice lo mismo y se lee mejor.

import "fmt"

// Consentimiento es qué se le debe a la persona que está en la máquina.
type Consentimiento string

const (
	// ConsentimientoLibre: no se le avisa. Es lo correcto para un servidor sin nadie enfrente,
	// y es lo que Musubi hacía siempre — ahora es una DECISIÓN y no el único comportamiento.
	ConsentimientoLibre Consentimiento = "libre"
	// ConsentimientoAvisa: se le notifica que alguien entró. No puede negarse. Para una máquina
	// de la empresa donde la vigilancia está acordada pero no es un secreto.
	ConsentimientoAvisa Consentimiento = "avisa"
	// ConsentimientoPide: tiene que aceptar. Sin respuesta, no hay sesión. Para la máquina de un
	// cliente, o para cualquier escritorio con una persona real trabajando.
	ConsentimientoPide Consentimiento = "pide"
	// ConsentimientoProhibido: no se abre nunca, aunque la capacidad esté concedida. Es el
	// candado del dueño de la máquina, y por eso puede más que un permiso del administrador.
	ConsentimientoProhibido Consentimiento = "prohibido"
)

// nivel es la escala. Más alto = más restrictivo. Es privado: nadie de afuera tiene por qué
// depender de los números, sólo del orden que expresan.
var nivel = map[Consentimiento]int{
	ConsentimientoLibre:     0,
	ConsentimientoAvisa:     1,
	ConsentimientoPide:      2,
	ConsentimientoProhibido: 3,
}

// ConsentimientoPorDefecto es lo que rige cuando nadie declaró nada.
//
// ES `avisa` Y NO `libre`, y la diferencia importa. `libre` como default significa que agregar
// una máquina nueva a la flota la deja, en silencio, sin ninguna protección para quien la usa —
// y el que la agregó no tuvo que decidirlo. `avisa` es el default que se puede defender: cuesta
// nada, no bloquea a nadie, y si alguien quiere silencio tiene que escribirlo.
//
// NO es `pide`: un default que BLOQUEA convertiría cada alta de máquina en una sesión que no se
// abre por un motivo que nadie configuró, y eso enseña a poner `libre` en todos lados para que
// deje de molestar. Un default demasiado estricto termina en menos seguridad, no en más.
const ConsentimientoPorDefecto = ConsentimientoAvisa

// Valido dice si el valor es uno de los cuatro. El vacío NO es válido acá a propósito: en este
// dominio «no declarado» lo resuelve el default, no un cero que se cuele hasta la decisión.
func (c Consentimiento) Valido() bool {
	_, hay := nivel[c]
	return hay
}

// MasRestrictivo devuelve el que más protege. Es la ÚNICA forma de combinar dos fuentes.
//
// Un valor inválido —o vacío— NO se ignora ni se toma como `libre`: se lo trata como el default.
// Ignorarlo dejaría que un typo en un archivo de configuración («Pide» con mayúscula, «ask»)
// abra sesiones sin avisar, que es exactamente el modo de fallo que este eje viene a cerrar:
// una configuración que parece puesta y no lo está.
func MasRestrictivo(a, b Consentimiento) Consentimiento {
	na, nb := nivelDe(a), nivelDe(b)
	if na >= nb {
		return normalizar(a)
	}
	return normalizar(b)
}

func normalizar(c Consentimiento) Consentimiento {
	if c.Valido() {
		return c
	}
	return ConsentimientoPorDefecto
}

func nivelDe(c Consentimiento) int { return nivel[normalizar(c)] }

// ResolverConsentimiento acumula lo que dicen las fuentes, de la más general a la más específica.
//
// EL ORDEN DE LOS ARGUMENTOS NO IMPORTA, y eso es deliberado: no es una cascada donde lo más
// específico pisa a lo general —ése es el modelo de CSS y acá sería un agujero—. Es un máximo:
// si el proyecto dice `pide` y la máquina dice `libre`, gana `pide`. Una máquina no puede
// AFLOJAR lo que el proyecto endureció; sólo puede endurecerlo más.
//
// La consecuencia práctica: para que una máquina se pueda mirar sin avisar, TODAS las fuentes
// tienen que decir `libre`. Que sea trabajoso es el punto.
//
// EL DEFAULT SE APLICA POR AUSENCIA, NO COMO PISO, y la primera versión de esto lo tenía mal:
// arrancaba en `ConsentimientoPorDefecto` y tomaba el máximo, con lo cual `libre` quedaba
// INALCANZABLE — declararlo en todas las fuentes seguía dando `avisa`, contradiciendo el párrafo
// de arriba. La prueba lo cazó. Es la misma forma de todos los bugs de este track: dos partes
// que dicen cosas distintas sobre lo mismo, y la que gana no es la documentada.
func ResolverConsentimiento(fuentes ...Consentimiento) Consentimiento {
	var res Consentimiento
	visto := false
	for _, f := range fuentes {
		// Un valor ilegible NO se saltea: cuenta como el default. Saltearlo dejaría que un typo
		// en la fuente más restrictiva desaparezca sin dejar rastro.
		if !f.Valido() {
			f = ConsentimientoPorDefecto
		}
		if !visto {
			res, visto = f, true
			continue
		}
		res = MasRestrictivo(res, f)
	}
	if !visto {
		return ConsentimientoPorDefecto
	}
	return res
}

// PideAprobacion dice si hay que esperar un sí antes de abrir.
func (c Consentimiento) PideAprobacion() bool { return normalizar(c) == ConsentimientoPide }

// AvisaAlUsuario dice si hay que notificar. `pide` también avisa: preguntar es avisar y algo más.
func (c Consentimiento) AvisaAlUsuario() bool { return nivelDe(c) >= nivel[ConsentimientoAvisa] }

// Bloquea dice si la sesión no se abre pase lo que pase.
func (c Consentimiento) Bloquea() bool { return normalizar(c) == ConsentimientoProhibido }

// ErrConsentimientoProhibido es el techo declarado por el dueño de la máquina.
//
// Es un error propio y no un «sin permiso» a secas porque manda a mirar OTRO lado: la capacidad
// puede estar perfectamente concedida y la sesión igual no se abre. Confundirlos haría que
// alguien revise `principals.yaml` durante media hora buscando un permiso que ya está.
var ErrConsentimientoProhibido = fmt.Errorf("la máquina tiene el acceso interactivo PROHIBIDO por configuración de consentimiento: " +
	"no es un problema de permisos —la capacidad puede estar concedida— es el candado del dueño de la máquina")
