package mcp

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// REVOCAR UNA MÁQUINA RESOLVÍA SUS ALERTAS EN SILENCIO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `revoked` es una BANDERA y no un DELETE, así que la fila queda — pero la máquina sale del
// export, sus series se vuelven obsoletas, y TODAS sus alertas se resuelven solas. Del otro lado
// del canal, «se arregló» y «la sacamos del inventario» llegan como el MISMO `[RESOLVED]`.
//
// Quien lo lee concluye que el problema se atendió, y la máquina con el disco lleno que se dio de
// baja sin arreglar queda cerrada en la cabeza de todos. Es el mismo defecto que este track ya
// arregló dos veces con otros nombres: dos causas distintas produciendo la misma señal.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestUnaBajaRecienteSeAnunciaYDespuesDesaparece(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-que-se-va")
	latirComoEmisor(t, ts.URL, tok, "dddd4444")

	dump := func(ahora time.Time) string {
		var b strings.Builder
		renderFlota(&b, s.engine, nil, ahora, s.sondaIntervalo, "0.140.3", nil, serviciosPorProyectoDefault)
		return b.String()
	}

	// ANTES DE LA BAJA no hay nada que anunciar: una serie que existiera siempre convertiría el
	// aviso en parte del paisaje, que es otra forma de no decir nada.
	if strings.Contains(dump(time.Now()), nombreBajaReciente+"{") {
		t.Fatal("se anuncia una baja de una máquina que está viva")
	}

	if ok, err := s.engine.RevocarDevice("casa", "pc-que-se-va"); err != nil || !ok {
		t.Fatalf("no se pudo revocar: ok=%v err=%v", ok, err)
	}

	// RECIÉN DADA DE BAJA: se anuncia.
	salida := dump(time.Now())
	if !strings.Contains(salida, nombreBajaReciente+`{project="casa",device="pc-que-se-va"}`) {
		t.Errorf("una máquina recién revocada no se anuncia.\n"+
			"  Sus alertas se van a resolver solas y del otro lado del canal eso se lee como un "+
			"arreglo: «se arregló» y «la sacamos del inventario» llegan como el mismo [RESOLVED].\n%s",
			salida)
	}
	// Y NO VUELVE A LA FLOTA COMO SI ESTUVIERA VIVA: sus otras series siguen sin emitirse. Sin
	// este caso, la guarda la satisface meter las revocadas en el barrido principal — que las
	// devolvería enteras, con cpu, disco y servicios, o sea lo contrario de lo que esto dice.
	if strings.Contains(salida, `musubi_fleet_device_up{project="casa",device="pc-que-se-va"}`) {
		t.Error("la máquina revocada volvió al export con sus series normales: eso la devuelve a la " +
			"flota como si estuviera viva, y es lo contrario de lo que el aviso de baja afirma")
	}

	// PASADA LA VENTANA: desaparece sola. El aviso acompaña a las resoluciones y después se va.
	//
	// LA VENTANA SE APLICA EN DOS CAMINOS DISTINTOS, y los dos hacen falta:
	//
	//   · el SQL de `ProyectosConBajasRecientes`, que es el que corta para una credencial
	//     FEDERADA — ahí los proyectos se descubren por sus bajas, así que una vieja no aparece;
	//   · y el chequeo en Go, que es el único que corta para una credencial ACOTADA A UN
	//     PROYECTO: ahí el proyecto viene de `proyectosVisibles` y entra siempre, así que sin él
	//     una baja de hace un año seguiría anunciándose para siempre.
	//
	// Se prueban LOS DOS. Con uno solo, sacar el otro salía en verde — medido.
	t.Run("federada: la ventana la corta la consulta", func(t *testing.T) {
		if strings.Contains(dump(time.Now().Add(ventanaDeBajaReciente+time.Hour)), nombreBajaReciente+"{") {
			t.Error("el aviso de la baja sigue después de la ventana: una serie permanente convierte " +
				"el aviso en paisaje, y a la tercera vez que alguien lo ve deja de leerlo")
		}
	})
	t.Run("acotada a un proyecto: la ventana la corta el chequeo en Go", func(t *testing.T) {
		// Una credencial con proyecto propio: `proyectosVisibles` devuelve «casa» SIEMPRE, tenga
		// bajas recientes o no, así que la consulta no filtra nada y el único corte es el de Go.
		p := &Principal{
			Name: "op-casa", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
			Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
		}
		var b strings.Builder
		renderFlota(&b, s.engine, p, time.Now().Add(ventanaDeBajaReciente+time.Hour),
			s.sondaIntervalo, "0.140.3", nil, serviciosPorProyectoDefault)
		if strings.Contains(b.String(), nombreBajaReciente+"{") {
			t.Errorf("con una credencial acotada al proyecto, el aviso de la baja sigue después de "+
				"la ventana: para ella el proyecto entra siempre, así que el único corte es el "+
				"chequeo de edad en Go.\n%s", b.String())
		}
	})
	// Y LA BAJA RECIENTE SÍ LLEGA A ESA MISMA CREDENCIAL. Sin este caso, la guarda de arriba la
	// satisface una compuerta que no deje ver NINGUNA baja a las credenciales acotadas — que es
	// justo a las que más les importa la de su propio proyecto.
	t.Run("acotada a un proyecto: la baja reciente sí se anuncia", func(t *testing.T) {
		p := &Principal{
			Name: "op-casa", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
			Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
		}
		var b strings.Builder
		renderFlota(&b, s.engine, p, time.Now(), s.sondaIntervalo, "0.140.3", nil, serviciosPorProyectoDefault)
		if !strings.Contains(b.String(), nombreBajaReciente+`{project="casa",device="pc-que-se-va"}`) {
			t.Errorf("una credencial del proyecto no se entera de la baja de su propia máquina:\n%s", b.String())
		}
	})
}

// Y LA ALERTA QUE LA LEE.
func TestLaAlertaDeMaquinaRevocadaExiste(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-flota.yml")), " ")
	if !strings.Contains(reglas, nombreBajaReciente+" >=") {
		t.Errorf("ninguna alerta lee `%s`: la serie diría que la máquina salió del inventario y "+
			"nadie estaría escuchando, así que las resoluciones seguirían llegando solas", nombreBajaReciente)
	}
}
