package fleet

import "encoding/json"

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
	// Protocolo dice qué pasó con el CONTRATO, por el mismo motivo que `Muestra` y `Servicios`:
	// que la máquina lo vea desde su lado. Vacío = el capver declarado cae dentro de la banda del
	// cerebro y no hay nada que decir.
	//
	// EL LATIDO NO SE RECHAZA POR ESTO, y la decisión importa: una máquina rechazada desaparece de
	// la flota, y desaparecer se lee igual que «apagada». Queda visible, con su problema escrito.
	Protocolo string `json:"protocolo,omitempty"`
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

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL CUERPO DEL LATIDO TAMBIÉN ESTABA ESCRITO DOS VECES, Y ESO ES LO QUE ARREGLA ESTE TIPO
//
// La nota de arriba cuenta cómo la RESPUESTA vivía duplicada y qué costó. El PEDIDO tenía el
// mismo problema y se pasó por alto: el agente lo armaba como un `map[string]any` anónimo en
// cmd/musubi/agent.go y el cerebro lo leía con un struct privado en internal/mcp/fleet_http.go.
// Dos formas del mismo mensaje, sin nada que las ate — y `encoding/json` descarta en silencio lo
// que el receptor no declara, igual que del otro lado.
//
// Un map es todavía peor que un struct duplicado: no tiene ni nombres de campo que el compilador
// pueda mirar, así que un typo en la clave («capver» vs «cap_ver») compila, arranca y responde
// 200 con el campo perdido.
// ────────────────────────────────────────────────────────────────────────────────────────────

// CuerpoLatido es lo ÚNICO que un dispositivo puede mandar, y lo que NO tiene es el invariante
// B4/D5: no hay dónde poner un `device_id`, un `name` ni un `project`. Todos sus campos son cosas
// que la máquina sabe DE SÍ MISMA; ninguno dice QUIÉN ES. La identidad sale del token y de ningún
// otro lado, así que una máquina no puede reportar las métricas de otra ni aunque quiera.
//
// (La línea en blanco de arriba no es estética: sin ella el bloque de la nota queda pegado a este
// comentario y pasa a ser el doc del tipo, que entonces no empieza por su nombre —ST1021—.)
type CuerpoLatido struct {
	// Muestra viaja como RawMessage y NO como *fleet.Muestra para poder pesarla CRUDA: el techo
	// de la telemetría es suyo (MuestraMaxBytes ≈ 4 KiB) y tiene que seguir siendo suyo
	// aunque el cuerpo entero haya crecido para hacerle lugar al inventario de servicios. Con un
	// solo techo compartido, una muestra de 100 KiB entraría por la puerta que se abrió para las
	// units, y el tope de la telemetría se habría aflojado sin que nadie lo decidiera.
	Muestra json.RawMessage `json:"muestra"`
	// Version y Direccion son lo que la máquina sabe de SÍ MISMA y el cerebro no puede
	// averiguar solo: qué build del agente corre y por qué dirección se la alcanza.
	//
	// Que el device escriba en su propia fila NO rompe B4/D5. El invariante es que no puede
	// decir QUIÉN ES —eso sale del token—, no que no pueda decir CÓMO ESTÁ. Sin campos de
	// identidad acá, la única fila que estos valores pueden tocar es la del token presentado.
	Version string `json:"version"`
	// Capver es la versión de CAPACIDADES del protocolo que habla ESTE agente. El cerebro la
	// compara contra su banda [CapverMin, Capver] y contesta en `Protocolo` cuando queda afuera.
	//
	// NO ES LA VERSIÓN DEL PRODUCTO, y por eso viaja aparte de `Version`: dos builds con versiones
	// distintas pueden hablar el mismo contrato, y ése es justamente el punto —permite desplegar
	// sin romper—. Un agente anterior a esta pieza no manda el campo y queda en 0, que significa
	// «no declara» y NO «versión cero»: el cerebro lo trata como fuera de banda porque nadie
	// afirmó nada, no porque haya afirmado algo viejo.
	Capver    int    `json:"capver,omitempty"`
	Direccion string `json:"direccion"`
	// RustdeskID es el identificador PÚBLICO del cliente de pantalla (S6). No es un secreto: sin
	// la contraseña de sesión no sirve para entrar, y sin él quien mira no sabe a qué conectarse.
	RustdeskID string `json:"rustdesk_id"`
	// Servicios es QUÉ CORRE ADENTRO de esta máquina (S12): sus units, sus contenedores.
	//
	// No rompe B4/D5 por la misma razón que `version` y `direccion`: un fleet.ReporteServicio no
	// tiene NINGÚN campo de identidad —ni device, ni project, ni id— así que lo único que estas
	// filas pueden tocar es el inventario de la máquina del token presentado. Y los tags están en
	// castellano a propósito: `nombre`, no `name`.
	Servicios []ReporteServicio `json:"servicios,omitempty"`
	// PuedePreguntar es una CAPACIDAD MEDIDA por el agente (A57): si en esta máquina hay dónde
	// dibujar un diálogo Y con qué. No es configuración — un servidor sin escritorio no tiene
	// dónde, y afirmarlo desde un archivo haría que un `pide` prometa un permiso que nunca se va
	// a pedir.
	//
	// PUNTERO Y NO bool, y ésa es la diferencia que importa: un agente VIEJO no manda el campo, y
	// con un bool pelado eso sería indistinguible de un agente nuevo que midió y dijo que no. El
	// nil se saltea y conserva lo que hubiera; el `false` explícito SÍ escribe. Sin esto, la
	// primera flota con agentes mezclados vería a los viejos «declarando» que no pueden preguntar
	// cuando en realidad no opinaron.
	PuedePreguntar *bool `json:"puede_preguntar,omitempty"`
	// MotivoNoPreguntar dice POR QUÉ no puede, cuando no puede. Sin él, un `pide` endurecido a
	// `prohibido` en toda la flota es un cero sin explicación, y las tres causas posibles —no hay
	// escritorio, falta un paquete, el agente corre como servicio— se arreglan distinto.
	MotivoNoPreguntar string `json:"motivo_no_preguntar,omitempty"`
}
