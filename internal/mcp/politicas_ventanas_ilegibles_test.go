package mcp

import (
	"errors"
	"testing"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// almacenSinVentanas es la base de siempre con UNA sola consulta rota: la de las ventanas de
// mantenimiento. Cuenta cuántas veces se la preguntaron, para que la prueba pueda exigir que los
// dos lados de la decisión hayan pasado de verdad por la rama de error.
//
// EMBEBE EL ALMACÉN REAL Y NO NIL, al revés que `backendConListaIlegible`: lo que se mide pasa
// DESPUÉS de la falla —el inventario tiene que listar la máquina y el barrido tiene que poder
// encolar—, y con un almacén nil la prueba moriría de un panic sin medir nada.
type almacenSinVentanas struct {
	memory.StorageBackend
	consultas *int
}

func (a almacenSinVentanas) DevicesEnMantenimiento(time.Time) (map[string]bool, error) {
	*a.consultas++
	return nil, errors.New("simulado: la consulta de ventanas de mantenimiento no contestó")
}

// A131 · revisión 2 · CON LAS VENTANAS ILEGIBLES, EL INVENTARIO DICE LO QUE HACE EL BARRIDO.
//
// ventanasParaPoliticas dice de sí misma «ES UNA SOLA FUNCIÓN PARA QUE EL SESGO SEA UNO SOLO»: si
// la consulta de ventanas falla, el barrido (aplicarPoliticas) y el inventario (musubi_fleet_list →
// porQueNoActuaria) tienen que equivocarse PARA EL MISMO LADO. Si el inventario leyera las ventanas
// por su cuenta y ante el error eligiera «en la duda, inerte» —que suena prudente—, diría
// `puede_actuar: false` de una política que el barrido está ejecutando: la contradicción entre
// indicador y acción que A131 vino a cerrar, escondida en la rama de error, que es la que nadie
// prueba.
//
// Y ESA FRASE NO TENÍA GUARDA. La revisión 2 de A131 hizo exactamente eso —el inventario leyendo
// por su cuenta y eligiendo el otro sesgo— y midió con una sonda `puede_actuar=false
// inerte_por=mantenimiento actuó=true`, con la suite entera de internal/mcp en verde salvo el
// censo (y el censo sólo porque el sabotaje le había tocado el `de` a otra directiva).
//
// LO QUE SE EXIGE ES LA IGUALDAD, NO UN SESGO. Qué lado elegir ante el error es una decisión
// (hoy: seguir como si no hubiera ventanas, que es como se comportaba el auto-heal antes de que
// existieran); lo que no puede pasar es que el indicador y la acción elijan distinto. Por eso la
// aserción es `puede_actuar == actuó` y no «actuó» a secas: un cambio de sesgo hecho en los dos
// lados a la vez es legítimo y esta prueba lo deja pasar.
//
// La ventana que se abre es REAL: la consulta rota no la deja ver, y así cualquier camino por el
// que UN solo lado se entere de la verdad —otra consulta, un caché— también sale acá como
// contradicción. Sin ella, «no hay ventana» sería además cierto y los dos sesgos podrían coincidir
// por casualidad.
//
// Exposición: no es un defecto vivo. Hoy las dos lecturas son la misma función (medido en la
// revisión 2: con la consulta rota, el inventario dice `puede_actuar: true` y el barrido actúa).
// Lo que esta prueba cierra es que deje de ser cierto sin que nada se ponga rojo.
//
// Sabotaje: que el inventario lea las ventanas por su cuenta y, ante el error, elija el OTRO sesgo
// («en la duda, inerte»: todas las máquinas en ventana). Es el sabotaje de la revisión 2, escrito
// en un solo tramo para no tocar la línea que sabotea TestLaPoliticaActuaDondeSuPrincipalPodriaYElInventarioLoDice.
// arnes: archivo="internal/mcp/methods_fleet.go"
// arnes: de="\t\tenMantenimiento = s.ventanasParaPoliticas(ahora, \"tool\", \"musubi_fleet_list\")\n"
// arnes: a="\t\tif en, err := s.engine.DevicesEnMantenimiento(ahora); err != nil {\n\t\t\tenMantenimiento = map[string]bool{}\n\t\t\tfor _, proy := range proyectos {\n\t\t\t\tds, _ := s.engine.ListarDevices(proy, true)\n\t\t\t\tfor _, dd := range ds {\n\t\t\t\t\tenMantenimiento[dd.ID] = true\n\t\t\t\t}\n\t\t\t}\n\t\t} else {\n\t\t\tenMantenimiento = en\n\t\t}\n"
//
// Sabotaje: el espejo, del lado que actúa: que el BARRIDO lea las ventanas por su cuenta y, ante el
// error, dé a todas sus máquinas por en ventana.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tenMantenimiento := s.ventanasParaPoliticas(ahora, \"proyecto\", proyecto)\n"
// arnes: a="\tenMantenimiento, errVentanas := s.engine.DevicesEnMantenimiento(ahora)\n\tif errVentanas != nil {\n\t\tenMantenimiento = map[string]bool{}\n\t\tfor _, d := range devices {\n\t\t\tenMantenimiento[d.ID] = true\n\t\t}\n\t}\n"
func TestConLasVentanasIlegiblesElInventarioDiceLoQueHaceElBarrido(t *testing.T) {
	s, d := maquinaConPolitica(t, []string{"metrics", "exec"})
	s.buscarPrincipal = registroDePrueba(autoHeal())
	ahora := time.Now()
	if _, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
		DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
		Desde: ahora.Add(-time.Minute), Hasta: ahora.Add(time.Hour), Motivo: "migración de postgres",
	}); err != nil {
		t.Fatalf("AbrirMantenimiento: %v", err)
	}
	latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple

	consultas := 0
	s.engine = almacenSinVentanas{StorageBackend: s.engine, consultas: &consultas}

	// 1. LO QUE VE UNA PERSONA, con la consulta de ventanas rota.
	puede, inertePor, visible := politicaEnElInventario(t, s)
	delInventario := consultas
	// 2. LO QUE PASA, con la misma consulta rota: la cola de la máquina, no el contador.
	antes := comandosDePolitica(t, s)
	s.aplicarPoliticas("casa", ahora)
	actuo := comandosDePolitica(t, s) > antes

	// PISOS: si alguno de los dos lados no pasó por la consulta rota, esta prueba compararía dos
	// lecturas sanas y su verde no diría nada sobre la rama de error.
	if delInventario == 0 {
		t.Fatalf("musubi_fleet_list no consultó las ventanas de mantenimiento: el inventario no pasó por la rama de error y esta prueba no la mide")
	}
	if consultas == delInventario {
		t.Fatalf("aplicarPoliticas no consultó las ventanas de mantenimiento: el barrido no pasó por la rama de error y esta prueba no la mide")
	}
	if !visible {
		t.Fatalf("el detalle de la política no viajó a una credencial con exec:* sobre una máquina que admite exec: no hay `puede_actuar` que comparar")
	}

	if puede != actuo {
		t.Errorf("con la consulta de ventanas rota, el inventario dice `puede_actuar: %v` (inerte_por %q) y la "+
			"política actuó=%v: el indicador y la acción eligieron sesgos distintos ante el MISMO error, y el "+
			"panel contradice a lo que el barrido está haciendo justo el día que la base no contesta",
			puede, inertePor, actuo)
	}
}
