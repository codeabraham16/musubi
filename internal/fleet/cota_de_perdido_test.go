package fleet

// cota_de_perdido_test.go cierra los huecos que la auditoría A131 (tema T10) encontró en las guardas
// de la cota de `perdido` —EsperaMaxDeEntregado: cuánto puede estar `entregado` un comando vivo— y el
// defecto VIVO que había debajo.
//
// Las guardas viejas clavaban un eje en un valor cómodo:
//
//   - TestUnEntregadoQueNuncaReportaSeMuestraPerdido DERIVA el instante del muerto de la misma
//     constante que custodia (`EsperaMaxDeEntregado` y un minuto): con la cota en 24 h el muerto se
//     corre con ella y la guarda sigue verde (C4-m4). Nada acotaba la cota por arriba.
//   - TestElUltimoDeUnaTandaLargaNoSeMarcaPerdidoMientrasEspera clava la espera en 89 min, debajo de
//     los nueve de adelante, y deja sin mirar la franja en la que el último CORRE su propio timeout:
//     con la cuenta hecha sobre nueve (C4-m6) o sin el margen del reporte (C4-m7), el último de una
//     tanda llena se dibujaba perdido mientras corría, y todo verde.
//   - Y ninguna miraba QUÉ se lleva el agente: la cuenta suponía que todo lo entregado está acotado
//     por el timeout de un comando, y la shell no lo está (C4-LD1, el defecto vivo; el porqué entero
//     está en el doc de EsperaMaxDeEntregado).
//
// Acá la tanda se ARMA desde la fuente de cada techo —ShellVidaMax, que aplica el cerebro;
// ComandoTimeoutMax, que valida el cerebro y aplica el agente; EsperaDeCierreDelAgente, que usa el
// agente; ComandosPorEntregaMax, que respeta el transporte (lo mide internal/mcp)— y NUNCA desde
// EsperaMaxDeEntregado: derivar el escenario de la constante custodiada es lo que dejó verde a C4-m4.

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

// techoDeUnComando es lo más que un comando del host tiene ocupado al agente: su timeout máximo, que
// el cerebro valida (ValidarComando) y el agente aplica, más la espera de cierre de sus tuberías.
const techoDeUnComando = ComandoTimeoutMax + EsperaDeCierreDelAgente

// finesDeLaPeorTanda devuelve, para la tanda que más tarda en volver, cuánto después de la entrega
// TERMINA cada comando, sin el viaje del reporte: la shell primero —muere a lo sumo ShellVidaMax
// después de la entrega, porque su sesión se crea antes— y detrás ComandosPorEntregaMax-1 comandos
// que agotan cada uno su techo, en serie, que es como los atiende el agente.
func finesDeLaPeorTanda() []time.Duration {
	fines := []time.Duration{ShellVidaMax}
	for i := 1; i < ComandosPorEntregaMax; i++ {
		fines = append(fines, fines[i-1]+techoDeUnComando)
	}
	return fines
}

// LA COTA DE `perdido` ES LA PEOR TANDA QUE EL AGENTE PUEDE DEVOLVER, NI UN SEGUNDO MENOS NI UNO MÁS.
//
// Se recorre el eje entero de «cuánto lleva entregado», con cada comando de la peor tanda: cada
// treinta segundos desde la entrega hasta que su reporte pudo llegar, y a un segundo de ese borde,
// está `entregado`; hasta que pudo llegar el reporte del ÚLTIMO de la tanda, también —la fila no sabe
// qué lugar ocupó—; y un segundo después de eso, `perdido`. Las dos mitades son los dos errores: la
// de abajo es el caro (un comando vivo dibujado muerto manda a relanzarlo) y la de arriba el que la
// auditoría midió con historia real (C4-m4: cuatro comandos que se perdieron de verdad se habrían
// visto «corriendo» un día entero).
//
// EXPOSICIÓN medida por la auditoría: de 1.807 comandos terminados, el que más tardó entre la entrega
// y el resultado tardó 9 minutos; la tanda más grande fue de 9 comandos, y hubo una sola fila de
// shell, de un minuto. Ningún `entregado` de hoy cae en la franja que estas mutaciones mueven: los
// cuatro que quedan llevan días y se ven `perdido` con cualquiera de las cotas.
//
// Sabotaje: la cuenta sin la shell, la de la base (C4-LD1) → la fila de una shell viva y lo que viaja
// detrás se dibujan perdidos desde los 102 minutos.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="ShellVidaMax +\n"
// arnes: a="ComandoTimeoutMax + EsperaDeCierreDelAgente +\n"
// arnes: colision_ok="TestUnaShellVivaYLoQueViajaDetrasNoSeDibujanPerdidos"
// Sabotaje: la cuenta sobre un comando de menos (C4-m6, en la cuenta de hoy) → el último de la tanda
// se dibuja perdido mientras corre su propio timeout.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="(ComandosPorEntregaMax-1)*"
// arnes: a="(ComandosPorEntregaMax-2)*"
// Sabotaje: la cuenta sin la espera de cierre del agente → el último se dibuja perdido dieciocho
// segundos antes de que su reporte pueda llegar.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="+EsperaDeCierreDelAgente)"
// arnes: a=")"
// Sabotaje: la cuenta sin el margen del reporte (C4-m7) → el último se dibuja perdido mientras su
// resultado viaja.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\t\tMargenDeReporte\n"
// arnes: a="\t\t0\n"
// Sabotaje: el «tarde y cierto» llevado al extremo (C4-m4) → un comando que se perdió de verdad se ve
// corriendo un día entero.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\tEsperaMaxDeEntregado = "
// arnes: a="\tEsperaMaxDeEntregado = 24*time.Hour + 0*"
func TestLaCotaDePerdidoEsLaPeorTandaRealNiMasNiMenos(t *testing.T) {
	entrega := time.Date(2026, 8, 31, 5, 0, 0, 0, time.UTC)
	fines := finesDeLaPeorTanda()
	// EL PISO: una tanda de uno no recorre la franja que la guarda vieja dejaba sin mirar.
	if ComandosPorEntregaMax < 2 || len(fines) != ComandosPorEntregaMax {
		t.Fatalf("la peor tanda tiene %d comandos y ComandosPorEntregaMax es %d: la prueba no arma la "+
			"tanda que dice", len(fines), ComandosPorEntregaMax)
	}
	// El último instante en que puede llegar un reporte legítimo de esa tanda.
	ultimoReporte := fines[len(fines)-1] + MargenDeReporte

	medidos := 0
	// Del último al primero: la primera falla que se lee es la del que más espera, que es la que la
	// guarda vieja dejaba sin mirar.
	for i := len(fines) - 1; i >= 0; i-- {
		fin := fines[i]
		c := Comando{
			Estado: EstadoEntregado, Argv: []string{"systemctl", "restart", "nginx"},
			Timeout: ComandoTimeoutMax, Entregado: entrega,
		}
		quien := fmt.Sprintf("el comando %d de %d de la tanda", i+1, len(fines))
		if i == 0 {
			c.Argv, c.Timeout = []string{OpShell, "ses-1", "24", "80"}, ComandoTimeoutDefault
			quien = "la fila del canal de una shell"
		}
		suReporte := fin + MargenDeReporte

		// Vivo mientras corre y mientras su reporte viaja. Se corta en la primera falla: con la cota
		// corta, cada instante de ahí en más repetiría la misma.
		edades := []time.Duration{}
		for edad := time.Duration(0); edad < suReporte; edad += 30 * time.Second {
			edades = append(edades, edad)
		}
		edades = append(edades, suReporte-time.Second, suReporte, ultimoReporte)
		for _, edad := range edades {
			medidos++
			if got := c.EstadoActual(entrega.Add(edad)); got != EstadoEntregado {
				porque := "todavía no pudo terminar y reportar"
				if edad >= suReporte {
					porque = "la fila no sabe qué lugar ocupó en la tanda, y hasta acá el último de la peor " +
						"tanda todavía puede reportar"
				}
				t.Errorf("%s, entregado hace %s, se muestra %q y tenía que seguir `entregado`: %s. Un comando "+
					"vivo dibujado muerto manda a relanzarlo (EsperaMaxDeEntregado=%s; su reporte puede llegar "+
					"hasta los %s y el último de la tanda hasta los %s)",
					quien, edad, got, porque, EsperaMaxDeEntregado, suReporte, ultimoReporte)
				break
			}
		}

		// Pasado el último reporte posible de la peor tanda, nadie lo espera más.
		for _, edad := range []time.Duration{ultimoReporte + time.Second, ultimoReporte + time.Hour} {
			if got := c.EstadoActual(entrega.Add(edad)); got != EstadoPerdido {
				t.Errorf("%s, entregado hace %s sin reporte, se muestra %q: ya pasó el último reporte posible de "+
					"la peor tanda (%s) y nadie tendría que seguir esperándolo. Una cota más larga que el peor "+
					"caso deja «corriendo» lo que se perdió (EsperaMaxDeEntregado=%s)",
					quien, edad, got, ultimoReporte, EsperaMaxDeEntregado)
			}
		}
	}
	// EL PISO del eje: la franja que la guarda vieja no miraba —del minuto 89 al último reporte— se
	// recorre de a treinta segundos con cada comando; por debajo de esto el barrido no la cruzó.
	if piso := int(ultimoReporte / (30 * time.Second)); medidos < piso {
		t.Fatalf("se midieron %d instantes y la franja pide al menos %d: el barrido no recorrió el eje", medidos, piso)
	}
}

// techoDeOperacion es lo más que UN comando de una operación interna tiene ocupado al agente cuando
// todo anda bien, y quién pone ese techo.
type techoDeOperacion struct {
	techo time.Duration
	quien string
}

// cuantoOcupaAlAgente es la DECISIÓN, por operación interna, de su techo. Va escrita a mano porque es
// una decisión sobre el diseño y no un derivado; lo que se deriva es QUÉ operaciones existen
// (opsInternasDeclaradas, del bloque const), y TestCadaOperacionInternaTieneDecididoCuantoOcupaAlAgente
// exige que las dos listas sean la misma.
func cuantoOcupaAlAgente() map[string]techoDeOperacion {
	return map[string]techoDeOperacion{
		OpShell: {ShellVidaMax, "la sesión entera: el agente reporta recién al cerrarla, y el cerebro " +
			"la cierra a los ShellVidaMax de creada"},
		OpPreguntar: {AvisoTimeout, "el agente corta la pregunta a los AvisoTimeout (cmd/musubi/avisador.go)"},
		OpAvisar: {techoDeUnComando, "el agente corta el aviso a los diez segundos (cmd/musubi/avisador.go), " +
			"y el techo de un comando lo cubre"},
		OpPantalla: {techoDeUnComando, "aplica una contraseña y vuelve; la vida de la sesión la lleva un " +
			"temporizador del agente, no la tanda"},
	}
}

// CADA OPERACIÓN INTERNA TIENE DECIDIDO CUÁNTO OCUPA AL AGENTE, Y LA COTA DE `perdido` LAS CUBRE A
// TODAS.
//
// La cuenta de EsperaMaxDeEntregado suponía que todo lo que se lleva el agente está acotado por el
// timeout de un comando, y la shell no lo está: su fila queda `entregado` toda la sesión. Ninguna
// guarda miraba el CONJUNTO de lo que el canal entrega, así que la suposición era invisible
// (C4-LD1).
//
// Acá las operaciones salen del bloque const (opsInternasDeclaradas) y cada una tiene que tener su
// techo decidido: una quinta operación no entra sin que alguien diga cuánto bloquea al agente. Con
// cada una se arma la peor tanda que puede encabezar —ella primero y detrás ComandosPorEntregaMax-1
// comandos en su techo—, y tanto su fila como la del último siguen `entregado` hasta que sus
// reportes pudieron llegar.
//
// Y A LO SUMO UNA OPERACIÓN PASA DEL TECHO DE UN COMANDO, porque la cuenta la suma UNA vez por tanda:
// las shells comparten su plazo absoluto —mueren a los ShellVidaMax de creadas, y se crean antes de
// la entrega—, así que dos shells en una tanda no suman cuatro horas. Una segunda operación larga sin
// esa propiedad se sumaría una vez por comando y la cuenta habría que rehacerla: esto la frena.
//
// EXPOSICIÓN medida por la auditoría: una sola fila `musubi:shell` en la historia de producción, de
// un minuto; ninguna sesión pasó de 102 minutos, y hoy no hay ninguna abierta.
//
// Sabotaje: declarar una operación interna nueva sin decidir cuánto ocupa al agente → la cota podría
// no cubrirla y nadie se enteraría.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tOpPantalla  = \"musubi:pantalla\"  // aplicar la contraseña de una sesión de pantalla\n"
// arnes: a="\tOpGrabar = \"musubi:grabar\"\n\tOpPantalla  = \"musubi:pantalla\"  // aplicar la contraseña de una sesión de pantalla\n"
func TestCadaOperacionInternaTieneDecididoCuantoOcupaAlAgente(t *testing.T) {
	declaradas := opsInternasDeclaradas(t)
	// EL PISO: si el barrido no encontrara nada, las comparaciones de abajo no medirían nada.
	if len(declaradas) < 4 {
		t.Fatalf("el barrido encontró %d operaciones internas (%v); había 4 cuando se escribió esta "+
			"prueba: el barrido está roto", len(declaradas), declaradas)
	}
	decidido := cuantoOcupaAlAgente()
	for op, nombre := range declaradas {
		if _, ok := decidido[op]; !ok {
			t.Errorf("%s (%q) está declarada y nadie decidió cuánto tiene ocupado al agente: la cota de "+
				"`perdido` podría no cubrirla, y su fila —o lo que viaje detrás— se dibujaría muerta "+
				"mientras corre. Sumala a cuantoOcupaAlAgente", nombre, op)
		}
	}
	for op := range decidido {
		if _, ok := declaradas[op]; !ok {
			t.Errorf("cuantoOcupaAlAgente decide sobre %q y ninguna constante del paquete la declara", op)
		}
	}

	entrega := time.Date(2026, 8, 31, 5, 0, 0, 0, time.UTC)
	var largas []string
	for op, d := range decidido {
		if d.techo > techoDeUnComando {
			largas = append(largas, op)
		}
		propia := Comando{Estado: EstadoEntregado, Argv: []string{op, "x"}, Timeout: ComandoTimeoutDefault, Entregado: entrega}
		if edad := d.techo + MargenDeReporte; propia.EstadoActual(entrega.Add(edad)) != EstadoEntregado {
			t.Errorf("la fila de %q, a los %s de entregada, se muestra %q: puede estar ocupando al agente "+
				"hasta %s (%s) y su reporte viajando %s más (EsperaMaxDeEntregado=%s)",
				op, edad, propia.EstadoActual(entrega.Add(edad)), d.techo, d.quien, MargenDeReporte, EsperaMaxDeEntregado)
		}
		// El último de la peor tanda que esta operación encabeza: ella, y detrás ComandosPorEntregaMax-1
		// comandos que agotan su techo, el último incluido.
		fin := d.techo + time.Duration(ComandosPorEntregaMax-1)*techoDeUnComando
		ultimo := Comando{Estado: EstadoEntregado, Argv: []string{"systemctl", "restart", "nginx"}, Timeout: ComandoTimeoutMax, Entregado: entrega}
		if edad := fin + MargenDeReporte; ultimo.EstadoActual(entrega.Add(edad)) != EstadoEntregado {
			t.Errorf("el último de una tanda encabezada por %q, a los %s de entregado, se muestra %q: %q "+
				"ocupa al agente hasta %s (%s), los %d comandos de detrás %s cada uno, y el reporte viaja %s "+
				"(EsperaMaxDeEntregado=%s)",
				op, edad, ultimo.EstadoActual(entrega.Add(edad)), op, d.techo, d.quien,
				ComandosPorEntregaMax-1, techoDeUnComando, MargenDeReporte, EsperaMaxDeEntregado)
		}
	}
	sort.Strings(largas)
	if len(largas) != 1 || largas[0] != OpShell {
		t.Errorf("pasan del techo de un comando %v, y la cuenta de EsperaMaxDeEntregado suma UNA sola "+
			"operación larga por tanda —la shell, porque todas las de una tanda mueren en el mismo plazo "+
			"absoluto—. Una operación larga más, o una distinta, pide rehacer la cuenta", largas)
	}
}
