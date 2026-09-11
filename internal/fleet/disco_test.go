package fleet_test

import (
	"testing"

	"musubi/internal/fleet"
)

// LA REGLA DE LOS PARES: O LAS TRES COLUMNAS, O NINGUNA.
//
// Un TOTAL sin su USADO produce un 0 % ocupado que un panel dibuja como «disco vacío» y un
// operador lee como «acá no pasa nada». El colector de Linux —el que corre en el cerebro— fijaba
// el total sin condición, el usado con una, y el disponible sin ninguna; los de macOS y Windows
// sí aplicaban la regla. La cautela estaba en dos de los tres hermanos.
//
// ESTAS PRUEBAS EXISTEN PORQUE LA ARITMÉTICA SE MUDÓ A UN ARCHIVO SIN SUFIJO DE PLATAFORMA.
// Mientras vivía adentro de `colector_linux.go`, probarla desde acá era imposible: habría hecho
// falta un `statfs` incoherente de verdad, o sea un filesystem roto.
func TestLasTresColumnasDelDiscoVanJuntasONoVan(t *testing.T) {
	const kib = 1024

	t.Run("un statfs sano da las tres", func(t *testing.T) {
		col, ok := fleet.ColumnasDeDiscoUnix(1000, 300, 250, kib)
		if !ok {
			t.Fatal("un statfs coherente tiene que poder reportarse")
		}
		// Usado = Blocks - Bfree (lo que ocupan los archivos), NO Blocks - Bavail: la diferencia
		// es la reserva de root, y confundirlas ya se llevó un 29,8 % en esta misma máquina.
		if col.Total != 1000*kib || col.Usado != 700*kib || col.Disponible != 250*kib {
			t.Errorf("columnas mal calculadas: %+v", col)
		}
		// Y la propiedad que las hace TRES y no dos: usado + disponible < total, porque entre
		// medio está la reserva.
		if col.Usado+col.Disponible >= col.Total {
			t.Errorf("usado+disponible (%d) no deja lugar a la reserva de root sobre un total de %d",
				col.Usado+col.Disponible, col.Total)
		}
	})

	// EL CASO QUE EL COLECTOR DE LINUX DEJABA PASAR A MEDIAS.
	t.Run("bfree mayor que blocks: no se reporta NADA", func(t *testing.T) {
		col, ok := fleet.ColumnasDeDiscoUnix(1000, 1001, 250, kib)
		if ok {
			t.Errorf("un statfs incoherente se reportó igual: %+v.\n"+
				"Así quedaba un TOTAL sin su USADO, o sea un 0 %% ocupado que el panel dibuja como "+
				"disco vacío sobre la máquina donde corre el cerebro", col)
		}
		if col != (fleet.ColumnasDeDisco{}) {
			t.Errorf("con ok=false las columnas tienen que venir en cero, y vinieron %+v: media "+
				"columna escrita es peor que ninguna", col)
		}
	})

	t.Run("bavail mayor que blocks tampoco", func(t *testing.T) {
		if _, ok := fleet.ColumnasDeDiscoUnix(1000, 300, 1001, kib); ok {
			t.Error("un disponible mayor que el filesystem entero se reportó: la alerta que divide " +
				"disponible sobre total daría más de 1 y no dispararía nunca")
		}
	})

	// UN CERO NO ES UN DISCO DE CERO BYTES: es «no medí».
	t.Run("blocks o bsize en cero: no se reporta", func(t *testing.T) {
		if _, ok := fleet.ColumnasDeDiscoUnix(0, 0, 0, kib); ok {
			t.Error("un filesystem de 0 bloques se reportó como un disco de 0 bytes: toda alerta que " +
				"divida por el total se rompe, y todo panel lo dibuja lleno")
		}
		if _, ok := fleet.ColumnasDeDiscoUnix(1000, 300, 250, 0); ok {
			t.Error("un bsize de 0 convierte las tres columnas en cero y se reportó igual")
		}
	})

	// EL DISCO REALMENTE LLENO SÍ SE REPORTA, y ésta es la otra dirección: sin este caso, la
	// guarda la satisface una función que no reporte NUNCA — y entonces `DiscoLleno` no puede
	// dispararse, que es peor que el defecto original.
	t.Run("un disco de verdad lleno se reporta, con disponible en cero", func(t *testing.T) {
		col, ok := fleet.ColumnasDeDiscoUnix(1000, 0, 0, kib)
		if !ok {
			t.Fatal("un disco lleno de verdad NO se reportó: así `DiscoLleno` no puede dispararse nunca")
		}
		if col.Disponible != 0 || col.Usado != 1000*kib {
			t.Errorf("un disco lleno tiene que decir disponible=0 y usado=total, y dijo %+v", col)
		}
	})
}

func TestLasTresColumnasDeWindowsSiguenLaMismaRegla(t *testing.T) {
	t.Run("un volumen sano da las tres", func(t *testing.T) {
		col, ok := fleet.ColumnasDeDiscoWindows(1000, 400, 300)
		if !ok || col.Total != 1000 || col.Usado != 600 || col.Disponible != 300 {
			t.Errorf("columnas mal calculadas: %+v ok=%v", col, ok)
		}
	})
	t.Run("libre mayor que el total: nada", func(t *testing.T) {
		if _, ok := fleet.ColumnasDeDiscoWindows(1000, 1001, 300); ok {
			t.Error("un libre mayor que el total se reportó: el usado daría un underflow de uint64 y " +
				"la máquina aparecería con exabytes ocupados")
		}
	})
	// LA CUOTA DEL USUARIO PUEDE SER MENOR QUE EL LIBRE — es el análogo de la reserva de root — y
	// eso NO es incoherente. Una guarda que lo rechazara dejaría sin medir a toda máquina con
	// cuotas activas.
	t.Run("una cuota por debajo del libre es legítima", func(t *testing.T) {
		if _, ok := fleet.ColumnasDeDiscoWindows(1000, 400, 50); !ok {
			t.Error("un volumen con cuota de usuario se rechazó: las cuotas son el caso normal en " +
				"una Windows corporativa, y rechazarlas la deja sin telemetría de disco")
		}
	})
	t.Run("total en cero: nada", func(t *testing.T) {
		if _, ok := fleet.ColumnasDeDiscoWindows(0, 0, 0); ok {
			t.Error("un volumen de 0 bytes se reportó como medición")
		}
	})
}
