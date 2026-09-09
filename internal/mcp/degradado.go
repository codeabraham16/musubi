package mcp

// Modo degradado: el servidor que HABLA cuando la memoria no abrió.
//
// EL DEFECTO QUE TAPA, Y CÓMO SE MIDIÓ. Hasta acá, cualquier fallo al abrir la memoria mataba el
// daemon con os.Exit(1) y un mensaje por stderr. Medido el 2026-09-07 sobre una base marcada v999
// con un binario que llega a v47, mandándole al daemon un `initialize` y un `tools/list`:
//
//	exit=1
//	STDOUT (lo que ve el cliente MCP): 0 bytes
//	STDERR: Error al arrancar base de datos: el esquema de la base es más nuevo que este
//	        binario: la base está en el esquema v999 pero este binario solo llega a v47
//
// El diagnóstico existía y era bueno; salía por el único canal que el cliente MCP no le muestra
// al agente. Cero bytes de protocolo es indistinguible de «musubi no está instalado», y las dos
// situaciones piden acciones OPUESTAS: una se arregla actualizando el binario, la otra
// instalándolo. Un modo de falla que se disfraza de otro es peor que uno ruidoso.
//
// Y NO ES SÓLO EL ESQUEMA: por este mismo camino mueren mudos el disco lleno, la base corrupta,
// el permiso denegado y cualquier error futuro de NewDbEngine. El escalón los cubre a todos
// porque se pone donde falla la apertura, no donde falla una causa en particular.
//
// LAS TRES DECISIONES:
//
//  1. CONTESTA initialize. Es el único momento del protocolo en el que este binario puede decir
//     quién es, y ahora además dice que no puede trabajar y por qué.
//
//  2. LISTA EL MISMO CATÁLOGO, no uno vacío. Un catálogo vacío le enseña al agente «este
//     servidor no tiene nada», que es la misma mentira de antes con otra cara. El catálogo se
//     construye sin tocar la memoria (lo prueba CatalogFingerprint, que lo arma sobre un
//     McpServer sin inicializar), así que declararlo entero no cuesta nada y es cierto: las
//     tools existen, lo que falta es con qué trabajar.
//
//  3. RECHAZA toda tools/call con la causa adentro. No despacha: el handler tocaría un engine
//     nil y entraría en pánico, y el recover de Dispatch lo convertiría en «error interno
//     inesperado» — de vuelta a un mensaje que no dice nada. El error explícito lleva la versión
//     del binario y el texto de la causa, que es lo que alguien necesita para arreglarlo.

import (
	"context"
	"encoding/json"

	"musubi/internal/embedding"
)

// NewServidorDegradado arma el servidor que atiende cuando la memoria NO abrió.
//
// engine va en nil a propósito y no hay un engine falso de por medio: un stub que devuelva vacío
// dejaría al agente leyendo «no hay memoria sobre eso» —una respuesta que se lee como un dato— en
// vez de «no pude mirar». Es la misma trampa que documenta el DTO sin tags JSON.
//
// No recibe options: un servidor sin memoria no tiene qué hacer con la config de mantenimiento,
// grafo, cognición ni cuotas, y pasárselas invitaría a arrancar schedulers contra un engine nil.
// Sólo la versión, porque es lo único que este servidor sí puede afirmar sobre sí mismo.
func NewServidorDegradado(projectPath, version string, causa error) *McpServer {
	s := NewMcpServer(nil, projectPath, embedding.NoopProvider{}, WithVersion(version))
	s.degradado = causa
	return s
}

// metaDegradacion devuelve el sobre `musubi/degraded` para el handshake, o nil si este servidor
// está sano.
//
// Va en `_meta` por el mismo motivo que la identidad: la forma de serverInfo la fija el spec de
// MCP, y un cliente estricto se rompe con campos de más. Un cliente que no entiende `_meta` lo
// ignora y no pierde nada — igual se entera al primer tools/call, que es donde importa.
func (s *McpServer) metaDegradacion() map[string]interface{} {
	if s.degradado == nil {
		return nil
	}
	return map[string]interface{}{
		// El texto crudo de la causa, sin resumir: es el que trae el número de esquema de los
		// dos lados, o el errno del disco.
		"reason": s.degradado.Error(),
		// Explícito y no derivable de la presencia del sobre: un cliente que agregue campos acá
		// mañana no debería tener que adivinar que existir significa «no sirve».
		"tools_usable": false,
	}
}

// errorDegradado es la respuesta a toda tools/call mientras la memoria no abra.
//
// El mensaje nombra las tres cosas que hacen falta para actuar: QUÉ binario es (para saber si hay
// que actualizarlo), QUÉ pasó (la causa textual) y que el binario está vivo (para que nadie salga
// a reinstalar lo que ya está instalado).
func (s *McpServer) errorDegradado() *RpcError {
	v := s.version
	if v == "" {
		v = "unknown"
	}
	return rpcErrorf(codeDegraded,
		"musubi %s está sirviendo en MODO DEGRADADO: no pudo abrir la memoria de este proyecto (%v). "+
			"El binario está instalado y respondiendo; ninguna tool musubi_* va a funcionar hasta que eso se resuelva.",
		v, s.degradado)
}

// interceptarDegradado corta toda tools/call antes del despacho cuando el servidor está
// degradado. Devuelve ok=false en un servidor sano, que sigue por el camino de siempre.
//
// Está acá y no dentro de handleToolsCall para que el camino sano no gane ni una rama: en
// handleToolsCall es una sola línea al principio, y toda la semántica del escalón vive en este
// archivo.
func (s *McpServer) interceptarDegradado(_ context.Context, _ json.RawMessage) (*RpcError, bool) {
	if s.degradado == nil {
		return nil, false
	}
	return s.errorDegradado(), true
}
