package mcp

// El escalón de SÓLO LECTURA: entre «trabaja» y «no puede ni hablar».
//
// Con el modo degradado (degradado.go) el daemon dejó de morirse mudo, pero seguía sin poder
// hacer nada. Con el piso de lectura grabado en la base, hay un caso en el medio que sí se puede
// atender: la base la migró un binario más nuevo, pero su piso declara que ESTE binario la lee
// bien. Ahí la memoria sirve para consultarla aunque no se pueda tocar.
//
// LOS TRES ESTADOS, Y QUE SEAN TRES ES EL PUNTO:
//
//	sano          migra, lee y escribe
//	sólo lectura  lee; toda tool que muta se rechaza nombrando el motivo
//	degradado     no hay memoria; contesta el protocolo y rechaza todo
//
// Antes de esta serie los tres se contestaban igual —el proceso se moría— y el agente no tenía
// forma de distinguirlos, ni de distinguirlos de «musubi no está instalado».
//
// DE DÓNDE SALE LA VERDAD. `soloLectura()` se le PREGUNTA AL ENGINE, no se guarda en el servidor:
// quien impide escribir es `PRAGMA query_only` del lado de SQLite, y un booleano propio acá podría
// quedar en desacuerdo con la base que realmente se abrió. El campo `motivoSoloLectura` es sólo
// TEXTO explicativo —lo pasa main, que tiene el error con los dos números de esquema— y no
// participa de ninguna decisión.

import "context"

// WithSoloLectura carga el texto que explica POR QUÉ este servidor no puede escribir. No enciende
// el modo: el modo lo decide el engine. Sin esta opción el mensaje sigue siendo correcto, apenas
// menos preciso.
func WithSoloLectura(motivo string) Option {
	return func(s *McpServer) { s.motivoSoloLectura = motivo }
}

// soloLectura dice si la memoria de este servidor se abrió en el escalón de sólo lectura.
//
// Se resuelve por type assertion sobre la interfaz del backend y no por un campo propio, para que
// haya UNA sola fuente de verdad. Sobre un engine nil —el servidor degradado— la assertion da
// false sin explotar, que es la respuesta correcta: un servidor sin memoria no está «en sólo
// lectura», está en otra cosa, y el interceptor degradado ya cortó antes de llegar acá.
func (s *McpServer) soloLectura() bool {
	e, ok := s.engine.(interface{ EsSoloLectura() bool })
	return ok && e.EsSoloLectura()
}

// metaSoloLectura devuelve el sobre `musubi/readonly` para el handshake, o nil si este servidor
// puede escribir.
func (s *McpServer) metaSoloLectura() map[string]interface{} {
	if !s.soloLectura() {
		return nil
	}
	motivo := s.motivoSoloLectura
	if motivo == "" {
		motivo = "la memoria se abrió en sólo lectura"
	}
	return map[string]interface{}{
		"reason": motivo,
		// Cuáles SÍ se pueden llamar, para que el cliente no tenga que descubrirlo a fuerza de
		// errores. Es el mismo eje `readOnly` que gobierna la autorización por rol, así que no
		// hay una segunda lista que mantener sincronizada.
		"tools_usable": s.nombresDeSoloLectura(),
	}
}

// sirveEnSoloLectura es EL predicado del escalón, y el único lugar donde se decide.
//
// Una tool sirve si no muta (readOnly) o si su clase la declara lectura-con-escritura-incidental
// (roLeeConEscrituraIncidental). El default sale de readOnly, así que una tool nueva no entra al
// modo por olvido: hay que afirmarlo.
func (s *McpServer) sirveEnSoloLectura(nombre string) bool {
	return s.toolReadOnly[nombre] || s.toolRO[nombre] == roLeeConEscrituraIncidental
}

// nombresDeSoloLectura lista las tools VISIBLES que este servidor sigue aceptando. Sale del mismo
// registro que sirve tools/list y con la misma regla de visibilidad: una tool dormida no se
// anuncia acá tampoco, aunque siga siendo despachable si alguien la nombra.
func (s *McpServer) nombresDeSoloLectura() []string {
	todas := toolsAllEnabled()
	out := make([]string, 0, len(s.tools))
	for i := range s.tools {
		if s.tools[i].dormant && !todas {
			continue
		}
		if s.sirveEnSoloLectura(s.tools[i].Name) {
			out = append(out, s.tools[i].Name)
		}
	}
	return out
}

// interceptarSoloLectura corta una tools/call que mutaría, antes de que la escritura llegue a
// SQLite y vuelva como un error del motor a mitad de camino.
//
// NO ES LA GARANTÍA, Y ESO IMPORTA. Aunque este intercept no existiera, `PRAGMA query_only`
// rechazaría la escritura igual. Lo que agrega es un error que EXPLICA —dice que el servidor está
// en sólo lectura y por qué— en vez de un «attempt to write a readonly database» que el agente no
// puede accionar. Y llega antes de que un handler haga la mitad del trabajo.
func (s *McpServer) interceptarSoloLectura(_ context.Context, tool string) (*RpcError, bool) {
	if !s.soloLectura() || s.sirveEnSoloLectura(tool) {
		return nil, false
	}
	motivo := s.motivoSoloLectura
	if motivo == "" {
		motivo = "la memoria se abrió en sólo lectura"
	}
	return rpcErrorf(codeReadOnly,
		"%q escribe y este servidor está en SÓLO LECTURA: %s. Las tools de lectura funcionan normalmente.",
		tool, motivo), true
}
