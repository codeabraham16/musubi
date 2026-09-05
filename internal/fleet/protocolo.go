package fleet

// protocolo.go es el CONTRATO del latido: lo que el cerebro responde y lo que el agente lee.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO VIVE ACÁ Y NO UNA COPIA EN CADA LADO
//
// Estaba escrito dos veces —`respuestaLatido` en internal/mcp/fleet_http.go y un struct anónimo
// adentro de latir(), en cmd/musubi— y nada ataba las dos. El resultado no fue teórico:
//
//	· `token_nuevo` existía en el cerebro desde el día uno y NO en el agente. La rotación de token
//	  de la Ola 2 no podía completarse nunca, y el falso verde duró hasta que alguien fue a
//	  verificar el arco a mano.
//	· `servicios` existía en el cerebro con una razón escrita —«un bloque descartado en silencio es
//	  indistinguible de uno que nunca se mandó, y quien puede arreglarlo es justamente el que no ve
//	  los logs del cerebro»— y tampoco estaba en el agente. O sea que ese mecanismo, cuyo único
//	  propósito era que algo NO desapareciera en silencio, desaparecía en silencio.
//
// Las dos son la misma falla y no hay comentario que la evite: `encoding/json` descarta los campos
// que el receptor no declara, sin error y sin log. Un campo nuevo del lado del cerebro se veía
// exactamente igual —compila, arranca, responde 200— tuviera receptor o no.
//
// Con UN solo tipo, agregar un campo lo agrega en los dos lados a la vez. No es una preferencia de
// estilo: es mover una clase de defecto silencioso a algo que el compilador sabe.
// ────────────────────────────────────────────────────────────────────────────────────────────

// RespuestaLatido es lo que el cerebro contesta a POST /fleet/heartbeat.
//
// Deliberadamente pobre: no devuelve nada que no le pertenezca a ESA máquina. Los campos con
// `omitempty` son los que el cerebro manda sólo cuando aplican; el agente tiene que tratar el vacío
// como «no aplica» y nunca como un valor.
type RespuestaLatido struct {
	OK      bool   `json:"ok"`
	Device  string `json:"device,omitempty"`
	Project string `json:"project,omitempty"`
	// Comandos son los pedidos de ejecución que le tocan a ESTA máquina (S5). Viajan de vuelta
	// en la respuesta del latido, por el canal que el agente ya abre él mismo: poner al agente a
	// escuchar un puerto sería la superficie que este track viene evitando desde S2, y sería
	// inútil la mitad de las veces porque esa máquina está detrás de un NAT.
	Comandos []ComandoParaElAgente `json:"comandos,omitempty"`
	// Muestra dice qué pasó con la telemetría: "guardada", "descartada: <razón>" o vacío si el
	// agente no mandó ninguna. El agente lo imprime, así que un colector roto o una capacidad
	// que falta se ven DESDE LA MÁQUINA en vez de desaparecer en silencio en el cerebro.
	Muestra string `json:"muestra,omitempty"`
	// Servicios dice qué pasó con el inventario, por el MISMO motivo que `Muestra`: un bloque
	// descartado en silencio es indistinguible de uno que nunca se mandó, y quien puede arreglarlo
	// —el que administra ESA máquina— es justamente el que no ve los logs del cerebro. Vacío = el
	// agente no mandó ninguno.
	Servicios string `json:"servicios,omitempty"`
	// Motivo viaja SÓLO en el 401 y es el mismo texto para todos los rechazos (B3).
	Motivo string `json:"motivo,omitempty"`
	// TokenNuevo es la credencial de una rotación en curso, y viaja SÓLO mientras la rotación
	// está abierta y el agente todavía late con el token viejo (Ola 2).
	//
	// Es la única cosa que este canal manda que no es un comando, y por eso vale decir qué la
	// hace aceptable: no amplía lo que el cerebro puede pedirle a la máquina —no ejecuta nada—,
	// va por un canal que el agente ya abre él mismo, y deja de mandarse en cuanto el agente
	// late con ella. Si el agente no la guarda, sigue llegando; si el plazo vence, la rotación
	// se abandona y el token viejo sigue valiendo.
	//
	// Cómo la recibe el agente —y por qué el archivo guarda una LISTA de tokens y no uno— está en
	// cmd/musubi/agent_token.go.
	TokenNuevo string `json:"token_nuevo,omitempty"`
}

// ComandoParaElAgente es lo MÍNIMO que el agente necesita para ejecutar. No viaja quién lo pidió
// ni por qué: el agente no tiene nada que hacer con esa información, y todo lo que viaja a la
// máquina más expuesta de la flota es superficie.
type ComandoParaElAgente struct {
	ID         string   `json:"id"`
	Argv       []string `json:"argv"`
	TimeoutSeg int      `json:"timeout_seg"`
}

// ResultadoDeComando es lo que el agente reporta a POST /fleet/result: la otra mitad del mismo
// canal que la RespuestaLatido.
//
// ESTABA DECLARADO DOS VECES, igual que la respuesta del latido: `resultadoDeComando` en
// cmd/musubi/ejecutor.go y `cuerpoResultado` en internal/mcp/fleet_http.go, byte a byte iguales.
// Ahí la duplicación es más peligrosa que en la respuesta, porque el cerebro DECIDE con estos
// campos y no sólo los muestra:
//
//	· de `Stdout` sale la respuesta de permiso del eje `pide` (ver PrefijoRespuestaPermiso). Si el
//	  tag divergiera, TODA la flota en modo `pide` negaría cada sesión de pantalla con «el agente
//	  no tuvo con qué preguntar» — habiendo preguntado y habiendo recibido un sí.
//	· de `ExitCode` y `Error` sale si una sesión de pantalla se marca activa o fallida.
//
// Las dos roturas son SILENCIOSAS. Una divergencia en `ComandoID`, en cambio, es ruidosa: el
// cerebro contesta 403 y el agente lo reporta. Que el modo de falla más grave sea el más callado es
// exactamente por lo que esto vive en un solo lugar.
//
// ExitCode es PUNTERO a propósito: nil = «no terminó» y 0 = «terminó bien». Un exit≠0 es un
// RESULTADO; `Error` es sólo para fallos del CANAL (no se pudo lanzar, venció el timeout).
// Confundirlos haría que un grep que no encuentra nada —exit 1, normal— se lea como máquina rota.
type ResultadoDeComando struct {
	ComandoID string `json:"command_id"`
	ExitCode  *int   `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error"`
}

// PrefijoRespuestaPermiso marca el stdout del agente como una RESPUESTA DE PERMISO y no como texto
// suelto (eje `pide`). Un prefijo y no un stdout pelado: sin él, cualquier salida inesperada —un
// binario que imprime algo raro— se interpretaría como una respuesta.
//
// ESTABA DECLARADO DOS VECES, uno por lado, con el mismo valor y sin nada que los atara. El modo de
// falla si divergían no era un error: el CutPrefix del cerebro no matchea, la respuesta cae en
// RespuestaNoSePudo, y toda la flota en modo `pide` niega cada sesión diciendo «el agente no tuvo
// con qué preguntar». Sin error, sin 4xx, con el agente reportando exit 0.
//
// Y era peor de lo que parece por una asimetría: el cerebro hace TrimSpace del stdout ANTES del
// CutPrefix y del resto DESPUÉS, así que el espacio en blanco que ENVUELVE se absorbe — pero el
// espacio final que va ADENTRO del literal no. Si el agente perdiera ese espacio y mandara
// "musubi-permiso:concedida", se rompe; si lo perdiera el cerebro, sigue funcionando. Un modo de
// falla que depende de QUÉ LADO se equivoca es de los más difíciles de razonar.
const PrefijoRespuestaPermiso = "musubi-permiso: "
