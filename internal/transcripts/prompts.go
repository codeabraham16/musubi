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

// prefijosDeSistema son los comienzos de un prompt que no escribió una persona. Medido el
// 2026-09-26 sobre los transcripts de Musubi y Altura desde el 09-14: 474 prompts humanos, 192
// `<task-notification>` (el aviso de una tarea de fondo que terminó), 38 `<command-…>` y 35
// `<local-command-…>`. Los avisos disparan el hook del turno como si fueran un pedido (128 de los
// 192), y el recall busca memoria con el texto del aviso como consulta: uno trajo
// architecture/notifications porque el aviso decía «notification».
var prefijosDeSistema = []string{"<task-notification", "<command-", "<local-command", "<system-reminder"}

// EsDeSistema dice si un prompt es un aviso del sistema y no un pedido: empieza, sin contar los
// espacios iniciales, con `<task-notification`, `<command-`, `<local-command` o `<system-reminder`.
//
// MIRA EL TEXTO Y NO EL `origin` DEL TRANSCRIPT a propósito: el hook del turno recibe sólo el
// texto del prompt, y el medidor tiene que clasificar con el MISMO criterio que la compuerta que
// mide, o mediría otra cosa.
func EsDeSistema(prompt string) bool {
	p := strings.TrimLeftFunc(prompt, unicode.IsSpace)
	for _, pre := range prefijosDeSistema {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
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
