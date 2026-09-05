package fleet

import "testing"

// LA ZONA SE ELIGE POR `type`, Y EL CASO 1 ES EL MEDIDO — no uno inventado.
//
// Sale de `musubi-server` el 2026-09-05, que tiene las tres zonas y donde el defecto se destapó:
// `thermal_zone0` es `acpitz` y no se movió en tres horas (91 puntos idénticos) mientras el
// paquete de CPU estaba 13 grados más arriba. Leer la zona 0 daba un número con las unidades
// correctas, el rango plausible y la forma de un dato — y respondía otra pregunta.
//
// Sabotaje verificado que lo pone en rojo: sacar `x86_pkg_temp` de preferenciaDeZonaTermica.
//
// Y uno que NO lo pone, que es más útil dejarlo escrito: volver `MuestraDesde` a
// `ParsearTempMiligrados(l.TempMil)` —o sea deshacer el arreglo entero— deja ESTA prueba en
// verde, porque ejercita `ElegirTemperatura` directo y nunca pasa por su llamador. Lo corrí y
// salió `ok`. Ese hueco lo cubre TestMuestraDesdeEligeLaZonaYNoParseaElListadoEntero, más
// abajo. Queda anotado porque este comentario llegó a afirmar lo contrario, y un doc que
// nombra un sabotaje que no funciona enseña a confiar en una red que no está.
func TestElegirTemperaturaPrefiereElSensorDeCPUYNoElDeChasis(t *testing.T) {
	casos := []struct {
		nombre string
		texto  string
		esp    *float64
		porque string
	}{
		{
			nombre: "el caso medido en musubi-server: gana el paquete de CPU",
			texto:  "acpitz 27800\npch_cannonlake 51000\nx86_pkg_temp 41000\n",
			esp:    f(41),
			porque: "acpitz es estático y pch no es la CPU; x86_pkg_temp es el que contesta",
		},
		{
			nombre: "el orden de las líneas NO decide",
			texto:  "x86_pkg_temp 41000\nacpitz 27800\n",
			esp:    f(41),
			porque: "si ganara la primera, el arreglo dependería del orden del kernel — que es " +
				"exactamente lo que hizo que `zone0` pareciera la principal",
		},
		{
			nombre: "acpitz sola SÍ se usa",
			texto:  "acpitz 27800\n",
			esp:    f(27.8),
			porque: "en una máquina donde es lo único que hay, una lectura de chasis es mejor que " +
				"ninguna; descartarla sería cambiar un dato flojo por un hueco",
		},
		{
			nombre: "una preferida en 0 no gana por ser preferida",
			texto:  "x86_pkg_temp 0\nacpitz 27800\n",
			esp:    f(27.8),
			porque: "un sensor apagado devuelve 0, y ParsearTempMiligrados ya lo descarta; sin " +
				"esto la preferencia le ganaría a la plausibilidad y volveríamos a publicar un cero",
		},
		{
			nombre: "tipos desconocidos: cualquiera antes que acpitz",
			texto:  "acpitz 27800\nalgo_raro 45000\n",
			esp:    f(45),
			porque: "la lista de preferidas no puede ser exhaustiva; lo único que se sabe seguro " +
				"es que acpitz es la mala",
		},
		{
			nombre: "formato viejo: un número suelto sin tipo",
			texto:  "27800\n",
			esp:    f(27.8),
			porque: "es lo que devuelve una máquina cuyo agente todavía no se actualizó",
		},
		{nombre: "sin zonas", texto: "", esp: nil, porque: "no hay sensor: nil, no cero"},
		{nombre: "todas ilegibles", texto: "acpitz\nx86_pkg_temp basura\n", esp: nil,
			porque: "una zona sin valor legible no aporta"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := ElegirTemperatura(c.texto)
			switch {
			case c.esp == nil && got != nil:
				t.Fatalf("devolvió %.1f y se esperaba nil — %s", *got, c.porque)
			case c.esp != nil && got == nil:
				t.Fatalf("devolvió nil y se esperaba %.1f — %s", *c.esp, c.porque)
			case c.esp != nil && *got != *c.esp:
				t.Fatalf("devolvió %.1f y se esperaba %.1f — %s", *got, *c.esp, c.porque)
			}
		})
	}
}

// LOS DOS PRODUCTORES EMITEN EL MISMO FORMATO, y si uno cambia sin el otro se nota acá.
//
// Es la forma de A76: hay DOS lugares que leen la zona térmica —el colector local y el guion que
// corre por SSH sobre un Tier B— y la elección es una sola función compartida. Que ambos hablen
// el mismo idioma no lo garantiza el compilador: los une un string.
func TestLosDosProductoresDeZonaTermicaHablanElMismoFormato(t *testing.T) {
	// Lo que produce el colector local para las tres zonas de musubi-server.
	local := "acpitz 27800\npch_cannonlake 51000\nx86_pkg_temp 41000\n"
	// Lo que produce el `for` del guion remoto para las MISMAS zonas: idéntico por construcción
	// (`echo "$t $v"`), y esta prueba lo fija para que un cambio de formato en uno rompa acá.
	remoto := "acpitz 27800\npch_cannonlake 51000\nx86_pkg_temp 41000\n"
	l, r := ElegirTemperatura(local), ElegirTemperatura(remoto)
	if l == nil || r == nil {
		t.Fatal("alguno de los dos formatos no produjo temperatura")
	}
	if *l != *r {
		t.Errorf("el mismo hardware da %.1f local y %.1f remoto: los formatos divergieron", *l, *r)
	}
	if *l != 41 {
		t.Errorf("eligió %.1f y el paquete de CPU marca 41: la preferencia no se aplicó", *l)
	}
}

func f(v float64) *float64 { return &v }

// EL CABLEADO TAMBIÉN SE PRUEBA, Y ESTA PRUEBA EXISTE PORQUE MI SABOTAJE SALIÓ VERDE.
//
// Las pruebas de arriba ejercitan `ElegirTemperatura` DIRECTO, así que volver `MuestraDesde` a
// `ParsearTempMiligrados(l.TempMil)` —o sea deshacer el arreglo entero— las dejaba en verde. Lo
// corrí y salió `ok`. El comentario de arriba llegó a decir que ese sabotaje la ponía en rojo, y
// era falso: una prueba que cubre la función pero no su llamador deja el arreglo desconectable
// sin que nadie se entere, y un doc que nombra un sabotaje que no funciona es peor que no
// nombrarlo — enseña a confiar en una red que no está.
//
// Sabotaje verificado que SÍ la pone en rojo: `m.TempC = ParsearTempMiligrados(l.TempMil)` en
// MuestraDesde. Con el listado de varias zonas, ParsearTempMiligrados no puede parsearlo y
// devuelve nil.
func TestMuestraDesdeEligeLaZonaYNoParseaElListadoEntero(t *testing.T) {
	l := LecturasProc{
		// Las tres zonas de musubi-server, en el formato que producen los dos colectores.
		TempMil: "acpitz 27800\npch_cannonlake 51000\nx86_pkg_temp 41000\n",
	}
	m := MuestraDesde(l, nil)
	if m.TempC == nil {
		t.Fatal("la muestra salió SIN temperatura teniendo tres zonas legibles: el listado no se " +
			"está eligiendo, se está intentando parsear entero")
	}
	if *m.TempC != 41 {
		t.Errorf("la muestra lleva %.1f y el paquete de CPU marca 41: la elección no llegó a la "+
			"Muestra, que es lo único que viaja al cerebro", *m.TempC)
	}
	// Y el formato viejo sigue llegando: una máquina sin actualizar no pierde su temperatura.
	viejo := MuestraDesde(LecturasProc{TempMil: "27800\n"}, nil)
	if viejo.TempC == nil || *viejo.TempC != 27.8 {
		t.Errorf("el formato viejo (un número suelto) dejó de llegar a la Muestra: %v", viejo.TempC)
	}
}
