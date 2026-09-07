package main

import (
	"io"
	"os"
)

// precompact.go dejó de ser un hook y quedó como SHIM DE COMPATIBILIDAD. No borrar sin leer esto.
//
// QUÉ PASÓ. `musubi precompact --hook-mode` nació el 2026-08-15 (#313) para avisar, justo antes de
// que la conversación se compactara, que había que bajar lo durable del tramo. Emitía el envelope
// estándar de Musubi:
//
//	{"hookSpecificOutput":{"hookEventName":"PreCompact","additionalContext":"..."}}
//
// y ese envelope NUNCA LLEGÓ. Claude Code valida `hookSpecificOutput.hookEventName` contra un enum
// en el que "PreCompact" no figura, así que descartaba el objeto entero. El evento PreCompact no
// admite inyectar contexto al modelo: la vía documentada para reinyectar es SessionStart con
// matcher "compact", que dispara DESPUÉS del resumen — o sea, tarde, que es exactamente lo que este
// hook existía para evitar.
//
// Tres semanas en verde sin hacer nada, y el test del hook tampoco avisó: verificaba NUESTRO json
// (que el campo dijera "PreCompact") en vez de verificar que el otro lado lo aceptara. Un test que
// espera el proxy en vez de la cosa. La guarda que impide repetirlo vive ahora en detect.go
// (eventosQueLlevanContexto) y el aviso se mudó al hook UserPromptSubmit, que sí llega
// (buildDurableNudge en turn.go), disparando por cantidad de turnos.
//
// POR QUÉ EL SUBCOMANDO SIGUE EXISTIENDO. Todo settings.json ya instalado sigue apuntando acá. Si
// el subcomando desapareciera, cada compactación fallaría con "comando desconocido", que es peor
// que el silencio. Así que se queda: drena stdin, no imprime nada y termina bien. Se puede borrar
// cuando ningún settings.json en circulación lo referencie.

// runPrecompact drena stdin y termina en silencio. Claude Code interpreta la salida vacía como
// "el hook no tiene nada que decir", que es la verdad: lo que tenía para decir se mudó al hook de
// turno. Drenar importa porque el otro lado escribe el payload del evento y cerrar el descriptor
// antes de tiempo le rompería la escritura.
func runPrecompact() {
	_, _ = io.Copy(io.Discard, os.Stdin)
}
