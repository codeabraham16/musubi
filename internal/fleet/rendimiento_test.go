package fleet

// Pruebas del RENDIMIENTO: qué hizo un servicio, no en qué estado está (fase 4).

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func ptr(n int) *int { return &n }

// rendimientoSano es el reporte típico del bot: un minuto, cuarenta y siete consultas.
func rendimientoSano() *Rendimiento {
	return &Rendimiento{
		VentanaSeg: 60, Atendidas: 47, Fallidas: 3,
		Desglose:      map[string]int{"ok": 41, "no_puedo": 3, "vacio": 3},
		LatenciaP95Ms: ptr(820), LatenciaMaxMs: ptr(1940),
	}
}

// EL CERO ACÁ ES UN DATO, Y ES EL DATO MÁS IMPORTANTE. Es al revés que en todo el resto del track.
//
// Un rendimiento con todo en cero significa «miré y no pasó nada», que es exactamente lo que
// distingue un bot CALLADO de un colector MUERTO. La distinción vive un nivel más arriba: nil es
// «no se midió». Si el cero fuera rechazado —o si viajara como nil— el latido del colector
// desaparecería y las dos situaciones se verían idénticas: sin datos.
//
// Sabotaje que la hace fallar: rechazar Atendidas == 0 en Valida.
func TestUnRendimientoEnCeroEsUnaMedicionYNoUnaAusencia(t *testing.T) {
	vacio := &Rendimiento{VentanaSeg: 60, Atendidas: 0, Fallidas: 0}
	if err := vacio.Valida(); err != nil {
		t.Fatalf("un minuto sin trabajo se rechazó: %v\n"+
			"  Ese reporte ES el latido del colector: sin él, «el bot no tuvo consultas» y "+
			"«el colector se murió» se ven igual.", err)
	}
	// Y se distingue de «no medido», que es el nil.
	var noMedido *Rendimiento
	if err := noMedido.Valida(); err != nil {
		t.Errorf("un rendimiento ausente se rechazó: la mayoría de los servicios de systemd no "+
			"miden trabajo y eso es normal: %v", err)
	}
	// La diferencia tiene que sobrevivir al viaje por JSON, que es como se guarda.
	crudo, err := json.Marshal(SaludServicio{Tomada: time.Now(), Estado: EstadoCorriendo, Rendimiento: vacio})
	if err != nil {
		t.Fatal(err)
	}
	var vuelta SaludServicio
	if err := json.Unmarshal(crudo, &vuelta); err != nil {
		t.Fatal(err)
	}
	if vuelta.Rendimiento == nil {
		t.Error("el rendimiento en cero desapareció al serializar: `omitempty` sobre el puntero " +
			"tiene que conservar un cero MEDIDO; si se pierde, el latido del colector no llega")
	}
	// Y el que NO se midió no aparece.
	crudo2, _ := json.Marshal(SaludServicio{Tomada: time.Now(), Estado: EstadoCorriendo})
	if strings.Contains(string(crudo2), "rendimiento") {
		t.Errorf("un servicio sin rendimiento igual lo serializa: %s", crudo2)
	}
}

// LAS FALLIDAS SON UN SUBCONJUNTO, NO UN TOTAL APARTE.
//
// Sin esta regla, «3 atendidas y 7 fallidas» pasa y la tasa de error da 233 % — que se lee como un
// bug del panel y no como lo que es: un reportante contando dos cosas distintas con los mismos
// nombres.
//
// Sabotaje que la hace fallar: quitar la comparación Fallidas > Atendidas.
func TestLasFallidasNoPuedenSuperarALasAtendidas(t *testing.T) {
	r := &Rendimiento{VentanaSeg: 60, Atendidas: 3, Fallidas: 7}
	err := r.Valida()
	if err == nil {
		t.Fatal("se aceptaron más fallidas que atendidas: la tasa de error daría 233 %")
	}
	if !strings.Contains(err.Error(), "subconjunto") {
		t.Errorf("el error no explica la relación entre los dos números: %v", err)
	}
	// Iguales SÍ: todo lo que atendió salió mal es un estado real, y el peor.
	todoMal := &Rendimiento{VentanaSeg: 60, Atendidas: 7, Fallidas: 7}
	if err := todoMal.Valida(); err != nil {
		t.Errorf("se rechazó «todo lo que atendí falló», que es un estado real y el peor: %v", err)
	}
}

// UNA TASA SOBRE CERO NO ES 0 %, ES LA AUSENCIA DE UNA TASA.
//
// Un 0 % de error pintado sobre un servicio que no atendió nada se lee como «todo perfecto», que
// es lo contrario de lo que hay que mirar.
//
// Sabotaje que la hace fallar: devolver (0, true) cuando Atendidas == 0.
func TestNoHayTasaDeErrorSinNadaQueMedir(t *testing.T) {
	if _, hay := (&Rendimiento{VentanaSeg: 60}).TasaDeError(); hay {
		t.Error("un servicio que no atendió nada devolvió una tasa de error: un 0 % ahí se lee " +
			"«todo perfecto» sobre algo de lo que no sabemos nada")
	}
	var nulo *Rendimiento
	if _, hay := nulo.TasaDeError(); hay {
		t.Error("un rendimiento no medido devolvió tasa")
	}
	tasa, hay := rendimientoSano().TasaDeError()
	if !hay {
		t.Fatal("con 47 atendidas no hubo tasa")
	}
	if quiere := 100 * 3.0 / 47.0; tasa != quiere {
		t.Errorf("tasa = %v, esperaba %v", tasa, quiere)
	}
}

// UNA LATENCIA SIN NADA QUE MEDIR NO ES CERO, ES NADA.
//
// Un colector que manda p95=0 con atendidas=0 no midió un percentil bajo: no midió. Dejarlo pasar
// mete un 0 en la serie que hunde cualquier promedio JUSTO en los minutos tranquilos — y el
// gráfico queda diciendo que el sistema anduvo rapidísimo cuando en realidad no anduvo.
//
// Sabotaje que la hace fallar: quitar la guarda de Atendidas == 0 con latencia.
func TestUnaLatenciaSobreCeroUnidadesSeRechaza(t *testing.T) {
	r := &Rendimiento{VentanaSeg: 60, Atendidas: 0, LatenciaP95Ms: ptr(0)}
	err := r.Valida()
	if err == nil {
		t.Fatal("se aceptó un p95 sobre cero unidades: ese 0 hunde el promedio en los minutos tranquilos")
	}
	if !strings.Contains(err.Error(), "percentil") {
		t.Errorf("el error no explica por qué: %v", err)
	}
	// Con cero atendidas y SIN latencia es el latido normal del colector: tiene que pasar.
	if err := (&Rendimiento{VentanaSeg: 60}).Valida(); err != nil {
		t.Errorf("el latido en cero sin latencia se rechazó: %v", err)
	}
}

// EL p95 NO PUEDE SUPERAR AL MÁXIMO. Es la aserción barata que atrapa al reportante que cruzó los
// dos campos, y cruzarlos es fácil: los dos son enteros de milisegundos.
//
// Sabotaje que la hace fallar: quitar la comparación entre p95 y máximo.
func TestElP95NoPuedeSuperarAlMaximo(t *testing.T) {
	r := &Rendimiento{VentanaSeg: 60, Atendidas: 10, LatenciaP95Ms: ptr(900), LatenciaMaxMs: ptr(400)}
	if err := r.Valida(); err == nil {
		t.Fatal("se aceptó un p95 mayor que el máximo: los dos campos están cruzados")
	}
	// Iguales SÍ: con pocas muestras el p95 ES el máximo.
	iguales := &Rendimiento{VentanaSeg: 60, Atendidas: 2, LatenciaP95Ms: ptr(400), LatenciaMaxMs: ptr(400)}
	if err := iguales.Valida(); err != nil {
		t.Errorf("se rechazó p95 == máximo, que con pocas muestras es lo normal: %v", err)
	}
}

// EL CONTEO SIN SU VENTANA NO SE PUEDE LEER. «47 atendidas» no significa nada sin saber en cuánto
// tiempo, y deducir la ventana del intervalo del reportante ata dos números que viven en archivos
// distintos y que alguien cambia por otra razón.
//
// Sabotaje que la hace fallar: aceptar VentanaSeg == 0.
func TestUnConteoSinVentanaSeRechaza(t *testing.T) {
	r := &Rendimiento{Atendidas: 47}
	err := r.Valida()
	if err == nil {
		t.Fatal("se aceptó un conteo sin ventana: 47 por minuto y 47 por día no son lo mismo")
	}
	if !strings.Contains(err.Error(), "tasa") {
		t.Errorf("el error no dice por qué hace falta: %v", err)
	}
	// Y un backfill disfrazado de reporte periódico tampoco entra.
	backfill := &Rendimiento{VentanaSeg: VentanaMaxSeg + 1, Atendidas: 1_000_000}
	if err := backfill.Valida(); err == nil {
		t.Error("se aceptó una ventana de más de un día: eso es un backfill entrando por el " +
			"camino del reporte periódico, y descoloca cualquier tasa")
	}
}

// EL DESGLOSE PUEDE SUMAR MENOS QUE EL TOTAL, PERO NUNCA MÁS.
//
// Menos significa «no supe clasificar todo», que es honesto y hay que dejarlo pasar: forzar que
// cierre empuja al reportante a inventar una categoría «otros» que no midió. Más significa que el
// desglose y el total cuentan cosas distintas, y entonces NINGUNO de los dos se puede usar.
//
// Sabotaje que la hace fallar: quitar la comparación total > Atendidas.
func TestElDesgloseSumaMenosOIgualPeroNuncaMas(t *testing.T) {
	menos := &Rendimiento{VentanaSeg: 60, Atendidas: 47, Desglose: map[string]int{"ok": 40}}
	if err := menos.Valida(); err != nil {
		t.Errorf("se rechazó un desglose que suma menos que el total; eso es «no supe clasificar "+
			"todo», y forzar que cierre inventa una categoría que nadie midió: %v", err)
	}
	mas := &Rendimiento{VentanaSeg: 60, Atendidas: 10, Desglose: map[string]int{"ok": 8, "error": 5}}
	if err := mas.Valida(); err == nil {
		t.Fatal("se aceptó un desglose que suma más que el total: los dos cuentan cosas distintas")
	}
}

// LAS CLAVES SON ENTRADA NO CONFIABLE Y TERMINAN COMO ETIQUETAS. Las elige quien reporta, con el
// vocabulario de SU dominio, y se dibujan en el panel. Un desglose sin tope es cardinalidad
// ilimitada en Prometheus y una tabla ilegible en la pantalla.
//
// Sabotaje que la hace fallar: quitar el tope DesgloseMax (o el de largo de clave).
func TestElDesgloseTieneTopeDeClavesYDeLargo(t *testing.T) {
	muchas := &Rendimiento{VentanaSeg: 60, Atendidas: 0, Desglose: map[string]int{}}
	for i := 0; i < DesgloseMax+1; i++ {
		muchas.Desglose[string(rune('a'+i))] = 0
	}
	if err := muchas.Valida(); err == nil {
		t.Errorf("se aceptaron %d resultados distintos: la cardinalidad de una etiqueta se paga",
			len(muchas.Desglose))
	}
	larga := &Rendimiento{VentanaSeg: 60, Atendidas: 1,
		Desglose: map[string]int{strings.Repeat("x", DesgloseClaveMax+1): 1}}
	if err := larga.Valida(); err == nil {
		t.Error("se aceptó una clave más larga que el tope")
	}
	vacia := &Rendimiento{VentanaSeg: 60, Atendidas: 1, Desglose: map[string]int{"  ": 1}}
	if err := vacia.Valida(); err == nil {
		t.Error("se aceptó una clave vacía: no se puede dibujar ni etiquetar")
	}
}

// UN RENDIMIENTO IMPOSIBLE SE LLEVA PUESTO EL REPORTE ENTERO, y eso es la decisión: guardar un
// estado bueno con una tasa de error del 233 % al lado deja al panel dibujando un servicio sano
// con un número imposible, y nadie sabe cuál de los dos mirar.
//
// Sabotaje que la hace fallar: no llamar a Rendimiento.Valida desde SaludServicio.Valida.
func TestUnaSaludConRendimientoImposibleSeRechazaEntera(t *testing.T) {
	s := SaludServicio{
		Tomada: time.Now(), Estado: EstadoCorriendo,
		Rendimiento: &Rendimiento{VentanaSeg: 60, Atendidas: 3, Fallidas: 7},
	}
	err := s.Valida()
	if err == nil {
		t.Fatal("una salud con un rendimiento imposible pasó entera: el panel dibujaría un " +
			"servicio corriendo con 233 % de error")
	}
	if !strings.Contains(err.Error(), "rendimiento") {
		t.Errorf("el error no dice qué parte falló: %v", err)
	}
	// Y la salud de siempre, sin rendimiento, sigue pasando: el campo es opcional.
	if err := (SaludServicio{Tomada: time.Now(), Estado: EstadoCorriendo}).Valida(); err != nil {
		t.Errorf("una salud de systemd sin rendimiento se rompió: %v", err)
	}
}

// LAS CLAVES SE NORMALIZAN SUMANDO, NO PISANDO. Dos claves que sólo diferían en espacios son la
// misma cosa contada dos veces; quedarse con la última perdería la otra mitad, y el desglose
// dejaría de cuadrar con el total sin que nada lo dijera.
//
// Sabotaje que la hace fallar: usar `limpio[k] = v` en vez de `limpio[k] += v`.
func TestRecortarNormalizaLasClavesSumandoYNoPisando(t *testing.T) {
	r := RecortarReporte(ReporteServicio{
		Nombre: "alturito", Salud: SaludServicio{Tomada: time.Now(), Estado: EstadoCorriendo,
			Rendimiento: &Rendimiento{VentanaSeg: 60, Atendidas: 10,
				Desglose: map[string]int{"ok": 4, " ok": 3, "ok ": 2, "  ": 5}}},
	})
	d := r.Salud.Rendimiento.Desglose
	if len(d) != 1 {
		t.Fatalf("quedaron %d claves tras normalizar: %#v", len(d), d)
	}
	if d["ok"] != 9 {
		t.Errorf("ok = %d, esperaba 9 (4+3+2): las claves con espacios se pisaron en vez de sumarse", d["ok"])
	}
	if _, hay := d["  "]; hay {
		t.Error("sobrevivió una clave vacía")
	}
}

// LAS CLAVES SALEN ORDENADAS. El orden de un map de Go es aleatorio POR DISEÑO, y un panel que
// reordena sus columnas en cada refresco es ilegible.
//
// Sabotaje que la hace fallar: devolver las claves sin ordenar.
func TestLasClavesDelDesgloseSalenOrdenadas(t *testing.T) {
	r := rendimientoSano()
	// Se repite: con una sola corrida un map de tres claves puede salir ordenado de casualidad.
	for i := 0; i < 50; i++ {
		got := r.ClavesDelDesglose()
		quiere := []string{"no_puedo", "ok", "vacio"}
		for j := range quiere {
			if got[j] != quiere[j] {
				t.Fatalf("orden %v, esperaba %v: las columnas del panel bailarían en cada refresco", got, quiere)
			}
		}
	}
	if (*Rendimiento)(nil).ClavesDelDesglose() != nil {
		t.Error("un rendimiento nil devolvió claves")
	}
}
