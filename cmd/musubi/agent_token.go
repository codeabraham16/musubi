package main

// agent_token.go es de dónde sale la credencial del agente y qué pasa cuando el cerebro le
// ofrece otra. Ola 2 del plan empresa, la mitad del AGENTE de la rotación en caliente.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL ARCHIVO NO GUARDA UN TOKEN: GUARDA LOS QUE ESTA MÁQUINA PUEDE PRESENTAR, EN ORDEN
//
// Esa es toda la idea, y no es una comodidad — es lo único que impide que una rotación se
// convierta en un apagón. El cerebro completa la rotación cuando LLEGA un latido con el token
// nuevo, y al completarla MATA el viejo. Entre esas dos cosas hay dos instantes en los que la
// máquina puede apagarse, y cada uno la deja presentando una credencial que el cerebro no
// reconoce:
//
//	· Se guarda el nuevo y la máquina muere antes de estrenarlo. La ventana de rotación vence,
//	  el cerebro ABANDONA (deja el viejo vivo, que es la decisión que evita el apagón del lado
//	  del cerebro) y la máquina vuelve presentando un nuevo que ya nadie conoce.
//	· El latido con el nuevo vuelve 200 —o sea el cerebro ya promovió y mató el viejo— y la
//	  máquina muere antes de anotarlo. Vuelve presentando el viejo, que acaba de morir.
//
// Reordenar no arregla las dos: sólo elige cuál de las dos ventanas se corre. Con una LISTA se
// arreglan las dos con el mismo mecanismo, porque el archivo nunca deja de contener el token que
// sirve:
//
//	1. El token nuevo se APENDEA. El archivo queda [viejo, nuevo] y ahí sí se estrena.
//	2. Al primer 200 el archivo se reescribe con el que funcionó, y sólo con ése.
//	3. Al arrancar se prueban en orden: si el primero da 401 y hay otro, se prueba el otro.
//
// Muerte en el primer instante: arranca con el viejo, que sigue vivo, y el archivo se colapsa
// solo. Muerte en el segundo: arranca con el viejo, da 401, prueba el nuevo, 200. Se recupera sin
// que nadie vaya a la máquina — que es el único punto de que exista la rotación.
//
// EL KILL-SWITCH NO SE AFLOJA. Revocar borra los DOS hashes del lado del cerebro, así que los dos
// tokens dan 401 y el agente se detiene igual que siempre (B5). El reintento es de UN intento por
// token que el archivo ya tenía, nunca contra el mismo token: no es una forma de golpear el
// lockout, es preguntar por la otra llave del mismo llavero.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/fleet"
)

// credencial es el llavero del agente y cuál está usando.
type credencial struct {
	// ruta es el archivo del que salieron y donde se persisten. Vacía cuando el token vino por
	// variable de entorno: ahí no hay dónde escribir, y una rotación NO se puede adoptar.
	ruta string
	// tokens son los que esta máquina puede presentar, en el orden del archivo.
	tokens []string
	// i es el que está usando; probado marca los que ya se intentaron en ESTA corrida, para que
	// el fallback de 401 sea de un intento por token y no un bucle.
	i       int
	probado []bool
}

// cargarCredencial resuelve de dónde sale el token.
//
// EL ARCHIVO GANA SOBRE LA VARIABLE, y no es una preferencia estética: es el único de los dos que
// puede rotar. Si están las dos puestas, la que manda es la que puede sobrevivir a la rotación.
//
// La variable sigue andando porque hay máquinas donde el token no puede vivir en un archivo —un
// contenedor con el secreto inyectado, un CI—. Ahí el agente late igual y lo único que pierde es
// poder completar una rotación; que eso se DIGA en voz alta cuando pasa está en adoptar().
func cargarCredencial() (*credencial, error) {
	if ruta := strings.TrimSpace(os.Getenv(envTokenFile)); ruta != "" {
		b, err := os.ReadFile(ruta) // #nosec G304 -- la ruta la elige quien instala la máquina, no una entrada remota
		if err != nil {
			return nil, fmt.Errorf("no se pudo leer %s=%q: %w", envTokenFile, ruta, err)
		}
		toks := tokensDeArchivo(string(b))
		if len(toks) == 0 {
			return nil, fmt.Errorf("%s=%q no tiene ningún token adentro", envTokenFile, ruta)
		}
		return &credencial{ruta: ruta, tokens: toks, probado: make([]bool, len(toks))}, nil
	}
	if t := strings.TrimSpace(os.Getenv(envToken)); t != "" {
		return &credencial{tokens: []string{t}, probado: make([]bool, 1)}, nil
	}
	return nil, nil
}

// Fuente dice de DÓNDE salió el token, y con eso si una rotación se puede adoptar.
//
// LA DECISIÓN VIVE EN UN SOLO LUGAR A PROPÓSITO: `cargarCredencial` ya eligió el camino y dejó la
// prueba en `ruta` —vacía cuando vino por variable, porque ahí no hay dónde escribir—. Volver a
// mirar el entorno para contestar esto sería una SEGUNDA decisión sobre lo mismo, y este repo pasó
// el día arreglando defectos de esa forma exacta: dos lugares que deciden igual hasta que uno
// cambia. Se deriva del hecho ya establecido.
func (c *credencial) Fuente() string {
	if c == nil {
		return ""
	}
	if c.ruta != "" {
		return fleet.CredencialDeArchivo
	}
	return fleet.CredencialDeVariable
}

// tokensDeArchivo parte el contenido en tokens, uno por línea.
//
// UN ARCHIVO DE UNA SOLA LÍNEA ES UN CASO VÁLIDO, y es lo que hay hoy en todas las máquinas: la
// lista es retrocompatible sin migrar nada. Las líneas vacías se ignoran para que un `echo` de más
// no invente un token vacío, que se presentaría y daría 401.
func tokensDeArchivo(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Actual es el token que el agente está presentando.
func (c *credencial) Actual() string { return c.tokens[c.i] }

// Usar devuelve el token del latido y lo marca como intentado en esta corrida.
func (c *credencial) Usar() string {
	c.probado[c.i] = true
	return c.tokens[c.i]
}

// Rechazado busca otro token del llavero después de un 401. Devuelve false cuando ya se probaron
// todos, que es el único caso en que un 401 significa «este dispositivo fue dado de baja».
func (c *credencial) Rechazado() bool {
	for j := range c.tokens {
		if !c.probado[j] {
			c.i = j
			return true
		}
	}
	return false
}

// Funciono colapsa el llavero al token que acaba de servir.
//
// Es la mitad que CIERRA la rotación del lado del agente: mientras el archivo tenga dos, el
// arranque puede elegir el que ya no vale y gastar un 401 en descubrirlo. Colapsar es barato y
// deja el estado en el que la próxima rotación vuelve a partir de uno.
func (c *credencial) Funciono() error {
	if len(c.tokens) == 1 || c.ruta == "" {
		return nil
	}
	bueno := c.tokens[c.i]
	if err := escribirTokens(c.ruta, []string{bueno}); err != nil {
		// NO es fatal: el token que sirve está en el archivo igual, junto al que no. La próxima
		// corrida gasta un 401 en descubrirlo y sigue andando. Decirlo alcanza.
		return err
	}
	c.tokens, c.i, c.probado = []string{bueno}, 0, []bool{true}
	return nil
}

// Sumar apendea el token de una rotación y lo deja seleccionado para el próximo latido.
//
// PERSISTE ANTES DE SELECCIONAR, y el orden es el invariante: si se seleccionara primero y la
// escritura fallara, el agente estrenaría un token que el archivo no tiene — y el cerebro lo
// promovería al recibirlo, matando el viejo, que es el único que quedaría en disco. Eso es el
// apagón. Con este orden, un fallo de escritura deja todo exactamente como estaba y la rotación
// simplemente no se completa: el cerebro la abandona y el viejo sigue valiendo.
func (c *credencial) Sumar(nuevo string) error {
	nuevo = strings.TrimSpace(nuevo)
	if nuevo == "" {
		return nil
	}
	for _, t := range c.tokens {
		if t == nuevo {
			return nil // el cerebro lo repite en cada latido de la ventana; ya lo tenemos
		}
	}
	if c.ruta == "" {
		return fmt.Errorf("el cerebro ofreció un token nuevo y no hay dónde guardarlo: "+
			"con %s no se puede completar una rotación (hace falta %s)", envToken, envTokenFile)
	}
	if err := apendarToken(c.ruta, nuevo); err != nil {
		return err
	}
	c.tokens = append(c.tokens, nuevo)
	c.probado = append(c.probado, false)
	c.i = len(c.tokens) - 1
	return nil
}

// apendarToken agrega un token al final del archivo y NO VUELVE HASTA QUE ESTÁ EN EL DISCO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO APENDEA EN VEZ DE REEMPLAZAR, QUE ES LO QUE HARÍA CUALQUIERA
//
// El reemplazo atómico de siempre —temporal, rename— crea una ENTRADA DE DIRECTORIO nueva, y esa
// entrada no es durable hasta que se sincroniza el DIRECTORIO. Sin ese segundo fsync, un corte
// duro puede llevarse el rename y revivir el archivo viejo, sin el token nuevo. Y ese es
// exactamente el único camino que el llavero no puede salvar: el cerebro ya promovió y mató el
// viejo, el archivo volvió sin el nuevo, y no hay otro que probar. Es la visita a la máquina que
// todo este diseño existe para evitar.
//
// Apendear no toca el directorio: el archivo YA existe, así que su entrada ya es durable, y con
// fsync del archivo alcanza. Vale igual en Linux y en Windows, donde el fsync de un directorio no
// está disponible — y no es un detalle de plataforma sino el caso que importa: la máquina que va a
// rotar lleva 14 cortes con BugcheckCode=0, o sea sin apagado ordenado.
//
// UN APPEND ROTO ES INOFENSIVO, que es la otra mitad del argumento. Si el corte parte la escritura,
// queda una línea con un token truncado: se presenta, da 401, y el llavero pasa al siguiente. Las
// líneas que ya estaban NO se tocan. Un rename roto, en cambio, se lleva el archivo entero.
// ────────────────────────────────────────────────────────────────────────────────────────────
func apendarToken(ruta, nuevo string) error {
	f, err := os.OpenFile(ruta, os.O_RDWR|os.O_APPEND, 0o600) // #nosec G304 -- ruta del instalador, no entrada remota
	if err != nil {
		return fmt.Errorf("no se pudo abrir %q para agregar el token: %w", ruta, err)
	}
	defer func() { _ = f.Close() }()

	// SI LO QUE HAY NO TERMINA EN SALTO, EL APPEND PEGARÍA LOS DOS TOKENS EN UNA LÍNEA y el
	// resultado sería un archivo con un token inválido y sin el viejo. Un `printf '%s'` del
	// instalador deja el archivo justo así, sin salto final.
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("no se pudo medir %q: %w", ruta, err)
	}
	prefijo := ""
	if fi.Size() > 0 {
		var ultimo [1]byte
		if _, err := f.ReadAt(ultimo[:], fi.Size()-1); err != nil {
			return fmt.Errorf("no se pudo leer el final de %q: %w", ruta, err)
		}
		if ultimo[0] != '\n' {
			prefijo = "\n"
		}
	}
	if _, err := f.WriteString(prefijo + nuevo + "\n"); err != nil {
		return fmt.Errorf("no se pudo agregar el token a %q: %w", ruta, err)
	}
	// EL fsync ES EL PUNTO DE TODA LA FUNCIÓN. Sin él los bytes quedan en page cache y el
	// llamador cree que la credencial está a salvo cuando todavía no lo está. Ordenar mal estas
	// líneas se ve idéntico a ordenarlas bien hasta el día del corte.
	if err := f.Sync(); err != nil {
		return fmt.Errorf("no se pudo sincronizar %q a disco: %w", ruta, err)
	}
	return nil
}

// escribirTokens reemplaza el archivo de forma atómica. Se usa SÓLO para colapsar el llavero.
//
// Acá sí conviene el temporal + rename, y acá sí alcanza sin fsync del directorio: si el corte se
// lleva el reemplazo, el archivo vuelve con los DOS tokens y el arranque gasta un 401 en descubrir
// cuál sirve. O sea que lo peor que pasa es lo que ya está declarado como no fatal en Funciono —al
// revés que en apendarToken, donde perder la escritura es perder la credencial.
//
// El temporal nace 0600 —os.CreateTemp lo hace— así que la credencial nunca existe con permisos
// amplios ni por un instante.
//
// EL MODO SE PRESERVA SI EL ARCHIVO YA ESTABA, porque el rename se lleva el del temporal y no el
// del destino: sin esto, una instalación que dejó el token 0400 terminaría en 0600 después de la
// primera rotación. Donde los bits no existen —Windows no mapea el modo POSIX— el Chmod no hace
// nada y el permiso lo tiene que dar la ACL del directorio, que es del instalador.
func escribirTokens(ruta string, tokens []string) error {
	dir := filepath.Dir(ruta)
	modo := os.FileMode(0o600)
	if fi, err := os.Stat(ruta); err == nil {
		modo = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".token-*")
	if err != nil {
		return fmt.Errorf("no se pudo crear el temporal del token en %q: %w", dir, err)
	}
	nombre := tmp.Name()
	defer func() { _ = os.Remove(nombre) }() // no-op si el rename ya lo movió
	if _, err := tmp.WriteString(strings.Join(tokens, "\n") + "\n"); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("no se pudo escribir el token: %w", err)
	}
	// Sync ANTES del rename: sin esto el rename puede quedar en el journal y el contenido no,
	// y un corte de luz deja un archivo que existe y está vacío.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("no se pudo sincronizar el token a disco: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("no se pudo cerrar el temporal del token: %w", err)
	}
	if err := os.Chmod(nombre, modo); err != nil {
		return fmt.Errorf("no se pudo fijar el modo del token: %w", err)
	}
	if err := os.Rename(nombre, ruta); err != nil {
		return fmt.Errorf("no se pudo reemplazar %q: %w", ruta, err)
	}
	return nil
}
