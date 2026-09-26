package fleet

// EstadosDeComando es el enum entero de EstadoComando, en el orden del bloque const (comando.go).
//
// EXISTE PARA QUE NADIE LO COPIE, igual que OrigenesDeComando. Lo recorren las pruebas del paquete
// que LEE la fila (internal/memory: qué deja escanearComando de ella, para cada estado) y del que la
// MUESTRA (internal/mcp: la bitácora y la cronología). Hasta A131 (tema T10) cada guarda de ésas
// clavaba el estado que miraba: la de las dos superficies probaba sólo `expirado`, y una cronología
// que derivaba `expirado` y nunca `perdido` pasaba en verde (C4-m5).
//
// TestLosEstadosDeComandoSonElEnumEntero la cierra contra el bloque const: un estado declarado que
// falte acá la pone en rojo.
var EstadosDeComando = []EstadoComando{EstadoPendiente, EstadoEntregado, EstadoTerminado, EstadoExpirado, EstadoPerdido}
