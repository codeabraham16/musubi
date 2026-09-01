package main

// alcance.go sondea, DESDE ESTA MÁQUINA, si se llega a los puertos que le declararon (A67).
//
// Existe porque el chequeo que había miraba el relay desde el propio servidor, y ése es el único
// punto de vista desde el que siempre anda. La pregunta de un relay es si un CLIENTE lo alcanza, y
// la única forma honesta de responderla es preguntándole al cliente.

import (
	"net"
	"os"
	"strings"
	"sync"

	"musubi/internal/fleet"
)

// envAlcance es la lista de destinos, separados por coma: `host:puerto,host:puerto`. Variable de
// entorno y no un flag, por la misma razón que el token y la URL del cerebro: mantener UNA forma
// de configurar la máquina vale más que la comodidad de un flag.
const envAlcance = "MUSUBI_ALCANCE"

// destinosDeAlcance se resuelve UNA vez por vida del proceso. La lista no cambia sola, y releer
// el entorno en cada latido sólo agregaría formas de que dos latidos midan cosas distintas.
var destinosDeAlcance = sync.OnceValue(func() []string {
	crudos := strings.Split(os.Getenv(envAlcance), ",")
	destinos, descartados := fleet.LimpiarDestinosDeAlcance(crudos)
	if descartados > 0 {
		// Una vez, y con el número: un destino mal escrito se descarta en silencio y después
		// alguien busca durante media hora por qué «la serie no aparece».
		avisarUnaVez("alcance-descartados",
			"%d destino(s) de MUSUBI_ALCANCE quedaron afuera: mal escritos, repetidos, o pasan el tope de %d. Formato: host:puerto",
			descartados, fleet.AlcanceMaxDestinos)
	}
	return destinos
})

// sondearAlcance prueba cada destino y devuelve una entrada por cada uno.
//
// EN PARALELO, y es una decisión con motivo: secuencial, cuatro destinos caídos son cuatro
// timeouts encadenados —12 s— adentro de un latido de 30. El peor caso pasa a ser UNA espera, no
// la suma. Son como mucho cuatro goroutines de vida corta (el tope lo pone el dominio), así que
// no hace falta un pool.
//
// SIN DESTINOS NO DEVUELVE NADA, ni una lista vacía de sondas: una máquina a la que nadie le pidió
// que mirara no puede quedar indistinguible de una que miró y no llegó.
func sondearAlcance() []fleet.SondaDeAlcance {
	destinos := destinosDeAlcance()
	if len(destinos) == 0 {
		return nil
	}
	sondas := make([]fleet.SondaDeAlcance, len(destinos))
	var wg sync.WaitGroup
	for i, d := range destinos {
		wg.Add(1)
		go func(i int, d string) {
			defer wg.Done()
			// UN TCP CONNECT Y NADA MÁS. No se manda ni un byte: el relay habla su propio
			// protocolo y escribirle basura sería, del lado de él, un cliente roto. La pregunta
			// es «¿me deja entrar la red?», y para eso el handshake alcanza.
			c, err := net.DialTimeout("tcp", d, fleet.AlcanceTimeout)
			if err == nil {
				_ = c.Close()
			}
			sondas[i] = fleet.SondaDeAlcance{Destino: d, Alcanza: err == nil}
		}(i, d)
	}
	wg.Wait()
	return sondas
}
