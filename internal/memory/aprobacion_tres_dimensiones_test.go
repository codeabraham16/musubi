package memory

import (
	"testing"
	"time"

	"musubi/internal/fleet"
)

// TestUnaAprobacionVigenteAcotaPorLasTresDimensiones — LA DIMENSIÓN QUE NADIE CUSTODIABA.
//
// El doc de `AprobacionVigenteDe` dice qué promete, y lo dice con tres nombres: «la aprobación se
// le dio a QUIEN pidió, para hacer ESO, en ESA máquina». La línea del contrato en `backend.go`
// nombra dos — «acota por solicitante Y por capacidad»— y la máquina se cae del enunciado. Y lo
// mismo pasaba con las guardas: `TestLaAprobacionDeUnoNoLeSirveAOtro` cubre el solicitante y
// `TestLaAprobacionDePantallaNoAbreUnaShell` la capacidad, las dos en `internal/mcp`. Por la
// máquina no preguntaba nadie.
//
// MEDIDO EL 2026-09-22, con control: poniendo `WHERE (device_id = ? OR 1=1)` en esa consulta,
// `./internal/mcp`, `./internal/memory` y `./internal/fleet` quedan los TRES en verde, y una
// aprobación concedida para una máquina abre otra. El «sí» se dio para entrar a UNA máquina.
//
// LA TABLA VARÍA UNA DIMENSIÓN POR FILA y deja las otras dos en su valor bueno, que es la única
// forma de saber cuál de las tres decidió. La fila de arriba es el control positivo: sin ella,
// una consulta que no encontrara NUNCA nada pasaría las tres de abajo con las mejores notas.
//
// Sabotaje que la hace fallar: sacarle el filtro por máquina a la consulta.
// arnes: archivo="internal/memory/aprobaciones.go"
// arnes: de="\t\t  WHERE device_id = ? AND solicitante = ? AND capacidad = ?"
// arnes: a="\t\t  WHERE (device_id = ? OR 1=1) AND solicitante = ? AND capacidad = ?"
func TestUnaAprobacionVigenteAcotaPorLasTresDimensiones(t *testing.T) {
	const (
		suMaquina     = "maquina-A"
		suSolicitante = "gio"
	)
	suCapacidad := fleet.CapScreen

	casos := []struct {
		nombre               string
		maquina, solicitante string
		capacidad            fleet.Cap
		quiero               bool
		porque               string
	}{
		{"las tres coinciden (control positivo)", suMaquina, suSolicitante, suCapacidad, true,
			"sin esta fila, una consulta que no encontrara nunca nada pasaría las otras tres"},
		{"OTRA máquina", "maquina-B", suSolicitante, suCapacidad, false,
			"el `sí` se dio para entrar a UNA máquina; si vale en otra, aprobar la de escritorio abre el servidor"},
		{"OTRO solicitante", suMaquina, "meirn", suCapacidad, false,
			"quien aprobó nombró a alguien: su `sí` era sobre esa persona"},
		{"OTRA capacidad", suMaquina, suSolicitante, fleet.CapShell, false,
			"aprobar que alguien MIRE una pantalla no es aprobar que abra una shell"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			e := nuevoEngineDePrueba(t)
			ahora := time.Now().UTC()
			sol, err := e.AbrirSolicitudDeAprobacion(fleet.SolicitudDeAprobacion{
				ID: "sol-1", DeviceID: suMaquina, ProjectID: "casa",
				Solicitante: suSolicitante, Capacidad: suCapacidad, Motivo: "entro",
				Estado: fleet.AprobacionPendiente,
				Creada: ahora, Vence: ahora.Add(10 * time.Minute),
			})
			if err != nil {
				t.Fatalf("AbrirSolicitudDeAprobacion: %v", err)
			}
			if ok, err := e.ResolverAprobacion(sol.ID, "revisora", "dale", true, ahora); err != nil || !ok {
				t.Fatalf("ResolverAprobacion: ok=%v err=%v", ok, err)
			}

			_, hay, err := e.AprobacionVigenteDe(c.maquina, c.solicitante, c.capacidad, ahora)
			if err != nil {
				t.Fatalf("AprobacionVigenteDe: %v", err)
			}
			if hay != c.quiero {
				t.Errorf("la aprobación de (%s, %s, %s) %s consultando (%s, %s, %s).\n  %s",
					suMaquina, suSolicitante, suCapacidad,
					map[bool]string{true: "SE ENCONTRÓ", false: "NO se encontró"}[hay],
					c.maquina, c.solicitante, c.capacidad, c.porque)
			}
		})
	}
}
