package fleet

// tanda.go es lo que el cerebro necesita saber de CÓMO ATIENDE EL AGENTE UNA TANDA para poder decir
// cuándo un comando `entregado` ya no va a volver (EsperaMaxDeEntregado, en comando.go).
//
// Vive en el dominio y no en el agente por la misma razón que ComandosPorEntregaMax: la cota de
// `perdido` lo suma, y un número que el agente usa y el cerebro copia es una cota que se
// desincroniza en silencio.

import "time"

// EsperaDeCierreDelAgente es el `WaitDelay` con el que el agente corre cada comando: cuánto más
// espera, después de matar uno que venció su timeout, a que se le cierren las tuberías.
//
// SIN ÉL EL TIMEOUT NO LIBERA AL AGENTE: un comando que deja un hijo en background se lleva las
// tuberías abiertas y `Run` no vuelve hasta que ese hijo las suelte (ver cmd/musubi/ejecutor.go).
// CON ÉL, cada comando de una tanda puede tardar su timeout Y ESTO, y la cota de `perdido` lo suma
// una vez por comando.
//
// ESTABA ESCRITO COMO UN `2 * time.Second` SUELTO EN EL AGENTE y la cuenta del cerebro no lo sumaba:
// con diez comandos eran veinte segundos que se comía, sin decirlo, el margen del reporte. Lo señaló
// la auditoría A131 (C4-m6 y C4-m7) al pedir que la guarda de la cota derive el peor caso de sus
// fuentes y no de la constante que custodia.
const EsperaDeCierreDelAgente = 2 * time.Second

// EsperaLargaDeShell es cuánto retiene el cerebro un pedido del relay de shell que no tiene nada
// que devolver: el GET vuelve apenas hay un byte y, si no hay ninguno, vuelve vacío a este plazo
// (long-poll). La usa el relay (internal/mcp/shell_relay.go) para los dos lados del canal.
//
// LA COTA DE `perdido` LA SUMA UNA VEZ POR TANDA, y por eso vive acá. Es lo más tarde que el agente
// se entera de que el cerebro le cerró la shell: el pedido de teclas que quedó colgado cuando la
// sesión venció vuelve vacío a este plazo, y recién el siguiente encuentra la sesión cerrada. Hasta
// ahí la tanda entera sigue parada detrás de la shell.
//
// ESTABA ESCRITA COMO UN `25 * time.Second` DEL RELAY, y la primera cuenta que sumó la shell (A131,
// tema T10) la dejaba adentro del margen del reporte, que con diez reportes en el borde de su corte
// no alcanzaba (ver MargenDeReporte). Lo señaló la revisión adversaria de ese tema.
const EsperaLargaDeShell = 25 * time.Second
