package memory

import "testing"

// Con el candado tomado por un proceso vivo, otro NO puede bajar: es lo que evita que dos terminales
// sobre la misma base bajen las mismas páginas en cada tick (medido: 1,7 veces el tráfico de un
// proceso en la laptop, 1,5 en Altura).
func TestReclamarBajadaUnSoloDueno(t *testing.T) {
	e := newTestEngine(t)
	if ok, err := e.ReclamarBajada("proc-a", 120); err != nil || !ok {
		t.Fatalf("el candado libre tenía que ser de proc-a; ok=%v err=%v", ok, err)
	}
	if ok, err := e.ReclamarBajada("proc-b", 120); err != nil || ok {
		t.Fatalf("con proc-a vivo, proc-b NO puede bajar; ok=%v err=%v", ok, err)
	}
}

// El dueño RENUEVA en cada tick en vez de soltar. Si soltara, el otro proceso bajaría en SU tick
// siguiente y los dos volverían a alternarse bajando lo mismo.
func TestReclamarBajadaElDuenoRenueva(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 3; i++ {
		if ok, err := e.ReclamarBajada("proc-a", 120); err != nil || !ok {
			t.Fatalf("tick %d: el dueño tiene que poder renovar; ok=%v err=%v", i, ok, err)
		}
	}
	if ok, _ := e.ReclamarBajada("proc-b", 120); ok {
		t.Fatal("tras renovar, el candado sigue siendo de proc-a")
	}
}

// Si el dueño muere, el candado vence solo y otro lo toma. Sin esto, cerrar la terminal que bajaba
// dejaría a la base sin bajada para siempre.
func TestReclamarBajadaVencidoLoTomaOtro(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SetMeta(metaBajadaLease, "proc-muerto|2000-01-01 00:00:00"); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.ReclamarBajada("proc-b", 120); err != nil || !ok {
		t.Fatalf("un candado vencido tiene que poder tomarlo otro; ok=%v err=%v", ok, err)
	}
	if ok, _ := e.ReclamarBajada("proc-muerto", 120); ok {
		t.Fatal("el que volvió de la muerte no puede recuperar un candado que ya es de otro")
	}
}

// Un dueño con prefijo común NO es el mismo dueño: la comparación es exacta, no por prefijo. Se
// prueban LAS DOS direcciones, y la que importa es la segunda. La primera versión de esta prueba sólo
// tenía la primera y SEGUÍA VERDE con el candado comparando por prefijo: si el guardado es el corto
// («proc-a») y reclama el largo, `LIKE 'proc-a-otro%'` no matchea igual. El agujero real es al revés:
// guardado el largo, reclama el corto, y `'proc-a-otro|…' LIKE 'proc-a%'` le regala el candado ajeno.
func TestReclamarBajadaNoConfundeDuenosConPrefijoComun(t *testing.T) {
	for _, caso := range []struct{ dueno, intruso string }{
		{"proc-a", "proc-a-otro"},
		{"proc-a-otro", "proc-a"}, // la dirección que muerde
	} {
		e := newTestEngine(t)
		if ok, _ := e.ReclamarBajada(caso.dueno, 120); !ok {
			t.Fatalf("setup: %s tenía que tomar el candado libre", caso.dueno)
		}
		if ok, _ := e.ReclamarBajada(caso.intruso, 120); ok {
			t.Errorf("«%s» no es «%s»: no puede renovar un candado ajeno", caso.intruso, caso.dueno)
		}
	}
}

// El cursor sólo avanza. Un tick más lento que el lease puede escribir después de que otro proceso
// tomó el candado vencido, y con SetMeta a secas el lento pisaba al rápido y el cursor retrocedía.
func TestAvanzarCursorBajadaNoRetrocede(t *testing.T) {
	e := newTestEngine(t)
	const k = "sync:inbound_cursor_prueba"
	for _, paso := range []struct {
		v    int64
		want string
	}{
		{100, "100"}, // arranca vacío: se escribe
		{50, "100"},  // un escritor lento con un cursor viejo: NO retrocede
		{150, "150"}, // avanza
		{150, "150"}, // igual: no cambia
		{9, "150"},   // un dígito menos: la comparación es NUMÉRICA, no de texto ("9" > "150")
	} {
		if err := e.AvanzarCursorBajada(k, paso.v); err != nil {
			t.Fatal(err)
		}
		if got, _, _ := e.GetMeta(k); got != paso.want {
			t.Fatalf("tras avanzar a %d el cursor quedó en %q; quería %q", paso.v, got, paso.want)
		}
	}
}
