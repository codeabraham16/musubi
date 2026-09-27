package main

import (
	"fmt"
	"io"
	"os"
)

// precompact.go — lo que Musubi le dice al resumen justo antes de que Claude Code compacte la
// conversación, para que el agente no pierda lo que importa.
//
// HAY DOS CANALES, Y SÓLO UNO LLEGA.
//
// El que NO llega. `musubi precompact --hook-mode` nació el 2026-08-15 (#313) para avisar, justo
// antes de compactar, que había que bajar lo durable del tramo. Emitía el envelope estándar de
// Musubi:
//
//	{"hookSpecificOutput":{"hookEventName":"PreCompact","additionalContext":"..."}}
//
// y ese envelope NUNCA LLEGÓ. Claude Code valida `hookSpecificOutput.hookEventName` contra un enum
// en el que "PreCompact" no figura, así que descartaba el objeto entero. Tres semanas en verde sin
// hacer nada: el test verificaba NUESTRO json en vez de verificar que el otro lado lo aceptara. La
// guarda que impide repetirlo vive en detect.go (eventosQueLlevanContexto) y el aviso de bajar lo
// durable se mudó al hook de turno (buildDurableNudge en turn.go).
//
// El que SÍ llega. En PreCompact, la salida estándar de un hook que termina en 0 se AGREGA a las
// instrucciones del resumen («stdout appended as custom compact instructions», contrato del binario
// 2.1.283). No es contexto para el modelo que trabaja: es lo que lee el que escribe el resumen.
// Medido el 2026-09-26 compactando una sesión de prueba con un hook que pedía una palabra testigo:
// el resumen registró «the requirement to end with CEREZA-7731».
//
// POR QUÉ IMPORTA AHORA. Desde que la instalación pone la ventana de compactación (ahorro_agente.go),
// la conversación se resume más seguido, y cada resumen es un punto donde el agente puede perder
// una regla que la persona le dio, el estado de una rama o de un despliegue, o el próximo paso. Estas
// instrucciones piden conservar eso textual. De paso piden no copiar salidas largas de comandos: el
// resumen queda en el contexto y se relee en cada pedido siguiente, así que cada línea de más se
// paga hasta la próxima compactación.
//
// EL SUBCOMANDO NO SE PUEDE BORRAR: settings.json ya instalados lo referencian, y sin él cada
// compactación fallaría con «comando desconocido».

// instruccionesDeCompactacion es lo que el resumen recibe de Musubi.
const instruccionesDeCompactacion = `Instrucciones de Musubi para este resumen (protegen el trabajo del agente):
- Conservá TEXTUALES las reglas, decisiones y pedidos de la persona, con su porqué, y marcá los pedidos que todavía no se cumplieron.
- Conservá el estado del trabajo: ramas, commits, PRs, despliegues y máquinas tocadas, cómo se verificó cada cosa, qué quedó a medias y el próximo paso.
- Conservá los errores que costaron caro y cómo se resolvieron, para no repetirlos.
- Conservá los ids de memoria de Musubi ([id:…]) que se usaron.
- No copies salidas largas de comandos o de tests: dejá el resultado. Del código, dejá la ruta, la función y qué cambió, y copiá sólo el fragmento que haga falta para seguir. Lo que está en el disco o en la memoria se puede volver a leer; lo que no quedó en el resumen se pierde.
`

// runPrecompact drena el payload del evento y le da al resumen las instrucciones de Musubi. Drenar
// importa: el otro lado escribe el payload, y cerrar el descriptor antes le rompería la escritura.
// Si el proyecto ya corre este hook desde su propio settings, el del plugin se calla: dos copias de
// las mismas instrucciones sólo agrandarían el pedido del resumen.
func runPrecompact() {
	_, _ = io.Copy(io.Discard, os.Stdin)
	if elPluginCedeElGancho("precompact") {
		return
	}
	fmt.Print(instruccionesDeCompactacion)
}
