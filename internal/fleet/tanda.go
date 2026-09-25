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
