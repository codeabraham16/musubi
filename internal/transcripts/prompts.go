package transcripts

import (
	"strings"
	"unicode"
)

// Origen dice quién escribió el texto que abrió un turno.
type Origen string

const (
	// OrigenApertura es el tramo que no abre un prompt: el principio del archivo o lo que sigue a
	// una compactación, hasta el primer prompt. Ahí caen los hooks de SessionStart.
	OrigenApertura Origen = "apertura"
	// OrigenHumano lo escribió una persona (o el orquestador, en una sesión hija).
	OrigenHumano Origen = "humano"
	// OrigenSistema es un aviso que llega por el mismo canal que un prompt y dispara el mismo hook,
	// pero no lo escribió nadie: ver EsDeSistema.
	OrigenSistema Origen = "sistema"
	// OrigenInterno lo escribe Claude Code en su propio transcript y NO pasa por ningún hook: el
	// resumen de una compactación y la marca de una interrupción.
	OrigenInterno Origen = "interno"
)

// avisoDeSistema es un comienzo de prompt que no escribió la persona.
type avisoDeSistema struct {
	prefijo string
	// loVeElHook dice si el hook del turno lo recibe. Los registros de un slash-command que Claude
	// Code resuelve sin el modelo NO: medido el 2026-09-26 en las sesiones principales de Musubi y
	// Altura, de toda la historia, 0 de 296 `<command-…>` y 1 de 283 `<local-command-…>` tienen un
	// bloque del hook detrás, contra 613 de 930 `<task-notification>`.
	loVeElHook bool
}

// avisosDeSistema es LA tabla de los prompts que no escribió la persona. La leen EsDeSistema —el
// medidor y la compuerta del hook del turno— y el aviso de tareas (cmd/musubi/tareas.go): eran dos
// tablas, discrepaban en cinco prefijos, y el mismo hook tenía dos definiciones de «lo escribió la
// persona».
//
// Son hechos del formato de Claude Code, medidos el 2026-09-26: `<task-notification` (terminó una
// tarea de fondo: 192 desde el 09-14 en Musubi y Altura, y el recall buscaba con el texto del aviso
// —uno trajo architecture/notifications porque el aviso decía «notification»—), `<cross-session-
// message` (un mensaje de otra sesión: 168 en los `queued_command` de la máquina), `<agent-message`
// (un subagente entrega su informe: 6), la forma presentada del mensaje de otra sesión («Another
// Claude session…»), `<system-reminder`, y los registros de un slash-command (`<command-…>` 38 y
// `<local-command-…>` 35 desde el 09-14). Se comparan tal como le llegan al hook: crudos.
var avisosDeSistema = []avisoDeSistema{
	{prefijo: "<task-notification", loVeElHook: true},
	{prefijo: "<cross-session-message", loVeElHook: true},
	{prefijo: "<agent-message", loVeElHook: true},
	{prefijo: "Another Claude session", loVeElHook: true},
	{prefijo: "<system-reminder", loVeElHook: true},
	{prefijo: "<command-", loVeElHook: false},
	{prefijo: "<local-command", loVeElHook: false},
}

// avisoDe devuelve el aviso con que empieza un prompt, sin contar los espacios iniciales.
func avisoDe(prompt string) (avisoDeSistema, bool) {
	p := strings.TrimLeftFunc(prompt, unicode.IsSpace)
	for _, a := range avisosDeSistema {
		if strings.HasPrefix(p, a.prefijo) {
			return a, true
		}
	}
	return avisoDeSistema{}, false
}

// EsDeSistema dice si un prompt es un aviso del sistema y no un pedido: empieza, sin contar los
// espacios iniciales, con alguno de los prefijos de avisosDeSistema.
//
// MIRA EL TEXTO Y NO EL `origin` DEL TRANSCRIPT a propósito: el hook del turno recibe sólo el
// texto del prompt, y el medidor tiene que clasificar con el MISMO criterio que la compuerta que
// mide, o mediría otra cosa.
func EsDeSistema(prompt string) bool {
	_, es := avisoDe(prompt)
	return es
}

// LoVeElHook dice si el hook del turno recibe este prompt: todo pedido de la persona y todo aviso
// salvo los registros de un slash-command (ver avisoDeSistema.loVeElHook). Un prompt que el hook no
// ve no puede recibir memoria, y contarlo en un denominador bajaría la tasa con cada slash-command.
func LoVeElHook(prompt string) bool {
	a, es := avisoDe(prompt)
	return !es || a.loVeElHook
}

// ClasificarPrompt dice quién escribió el texto de un registro que EsPrompt ya aceptó.
func ClasificarPrompt(reg *Registro, texto string) Origen {
	p := strings.TrimLeftFunc(texto, unicode.IsSpace)
	switch {
	case reg.IsCompactSummary,
		// Las versiones viejas no marcaban el resumen: se reconoce por cómo empieza.
		strings.HasPrefix(p, "This session is being continued"),
		strings.HasPrefix(p, "[Request interrupted"):
		return OrigenInterno
	case EsDeSistema(p):
		return OrigenSistema
	}
	return OrigenHumano
}

// palabrasDeContinuacion es la lista CERRADA de palabras con que el dueño pide seguir sin decir
// qué: «sigue», «go», «dale», «si hazlo». Va en minúsculas y sin acentos (se comparan plegadas).
// Es la de la línea base del frente arranque (2026-09-26): de 4.399 prompts del dueño, 1.212 son
// de pura continuación (continua 220, go 215, mira 154, sigue 105, listo 97…).
//
// NUNCA ENTRA UN TÉRMINO TÉCNICO. Un prompt corto y sustantivo que cae en la lista pierde su
// búsqueda de memoria, y por eso la cuida una prueba de tabla con prompts reales.
var palabrasDeContinuacion = conjunto(`
	sigue segui sigamos seguimos siga sigo continua continuemos continue go dale ok okay pk
	listo lista listos si no ya estamos esto eso esa ese todo toda bien perfecto genial va vamos
	hazlo hacelo haz hace hagamos mira ahi aca aqui entonces bueno gracias yes y e o hecho adelante
	procede avanza espera revisa vale claro correcto exacto tambien porfa favor me te lo le les se
	mi tu yo vos nos ahora igual asi mas pero muy que como esta estan hay`)

// vaciasDeContinuacion son las palabras vacías que tampoco dicen qué: artículos, preposiciones y
// sus pares en inglés.
var vaciasDeContinuacion = conjunto(`
	el la los las un una unos unas de del al en con por para que como su sus
	the an of in on at to for with and or is are be by as it`)

func conjunto(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// plegarAcentos le saca a una palabra en minúsculas las tildes y la diéresis de las vocales:
// «continúa», «seguí» y «mirá» se comparan como «continua», «segui» y «mira».
var plegarAcentos = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
)

// palabras parte un texto en tramos de letras o de dígitos, en minúsculas y sin acentos: «abc123»
// son dos palabras, y el guion bajo corta.
func palabras(s string) []string {
	var out []string
	var b strings.Builder
	clase := 0 // 0 nada, 1 letras, 2 dígitos
	cortar := func() {
		if b.Len() > 0 {
			out = append(out, plegarAcentos.Replace(b.String()))
			b.Reset()
		}
	}
	for _, r := range strings.ToLower(s) {
		c := 0
		switch {
		case unicode.IsLetter(r) || unicode.IsMark(r):
			c = 1
		case unicode.IsDigit(r):
			c = 2
		}
		if c != clase {
			cortar()
			clase = c
		}
		if c != 0 {
			b.WriteRune(r)
		}
	}
	cortar()
	return out
}

// EsPedidoDeContinuacion dice si un prompt pide seguir sin decir qué: sacando las palabras de la
// lista y las vacías, no queda ninguna palabra de 2 runas o más. «sigue» y «si hazlo» lo son;
// «hazlo global» y «mira en git», no. Un prompt sin texto (una imagen sola) no lo es: no pide
// seguir, trae otra cosa.
//
// ES DETERMINISTA Y SIN MODELO, y el hook del turno y el medidor lo comparten: si la compuerta
// usara una lista y el medidor otra, el número de después diría cuánto se parecen las listas.
func EsPedidoDeContinuacion(prompt string) bool {
	if strings.TrimSpace(prompt) == "" {
		return false
	}
	for _, w := range palabras(prompt) {
		if len([]rune(w)) > 1 && !palabrasDeContinuacion[w] && !vaciasDeContinuacion[w] {
			return false
		}
	}
	return true
}
