package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
	"musubi/internal/transcripts"
)

// El corrector de tipeo en el hook del turno (internal/memory/tipeo.go). El corrector mismo se
// prueba contra un motor real en internal/memory/tipeo_test.go; acá se fija lo que decide el TURNO:
// que el texto corregido es el que se embebe y se busca, cómo se avisa lo que se corrigió y que con
// la perilla apagada el bloque sale como antes.

const promptConTipeos = "la infromacion del comadno"

// corrigeDosTipeos hace de corrector de tipeo con dos correcciones fijas.
func corrigeDosTipeos(string) (string, []memory.Correccion) {
	return "la informacion del comando", []memory.Correccion{
		{Tipeado: "infromacion", Buscado: "informacion", DF: 7},
		{Tipeado: "comadno", Buscado: "comando", DF: 3},
	}
}

// embebedorQueAnota es un Provider que anota cada texto que le piden embeber.
type embebedorQueAnota struct{ textos *[]string }

func (e embebedorQueAnota) Embed(_ context.Context, s string) ([]float32, error) {
	*e.textos = append(*e.textos, s)
	return []float32{0.1, 0.2, 0.3}, nil
}
func (embebedorQueAnota) Dimensions() int { return 3 }
func (embebedorQueAnota) Name() string    { return "anota" }

func itemDelPi() memory.RecallResult {
	return memory.RecallResult{Count: 1, Items: []memory.RecallItem{{ID: "x1", TopicKey: "infra/pi", Gist: "El comando del pi", ContentHash: "h"}}}
}

// TestTurnoEmbebeLaConsultaCorregida: el turno corrige ANTES de embeber, y lo que va al embebedor y
// al recall es el texto corregido. Un caller que corrigiera y después embebiera el prompt crudo
// dejaría la mitad vectorial buscando el tipeo.
//
// Sabotaje: el turno embebe el prompt como vino.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\tvec, eerr := embedder.Embed(embCtx, consulta)\n"
// arnes: a="\t\tvec, eerr := embedder.Embed(embCtx, prompt)\n"
//
// Sabotaje: el turno busca el prompt como vino.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tres, err := store.Recall(context.Background(), consulta, opts)\n"
// arnes: a="\tres, err := store.Recall(context.Background(), prompt, opts)\n"
//
// Con qué vocabulario corrige lo fija TestTurnoCorrigeConElVocabularioDelModo.
func TestTurnoEmbebeLaConsultaCorregida(t *testing.T) {
	store := &fakeTurnStore{recall: itemDelPi(), corregir: corrigeDosTipeos}
	var embebidos []string

	buildTurnRecall(store, parametrosDelTurno{sesion: "s1", prompt: promptConTipeos, presupuesto: 250,
		memCfg: config.Default().Memory, embebedor: embebedorQueAnota{&embebidos}})

	if !store.llamoAlCorrector || store.corrigioConsulta != promptConTipeos {
		t.Fatalf("el turno tenía que pasarle el prompt al corrector: llamó=%v con %q", store.llamoAlCorrector, store.corrigioConsulta)
	}
	if len(embebidos) != 1 || embebidos[0] != "la informacion del comando" {
		t.Errorf("al embebedor tenía que ir el texto CORREGIDO, y fue %q", embebidos)
	}
	if store.lastQuery != "la informacion del comando" {
		t.Errorf("al recall tenía que ir el texto CORREGIDO, y fue %q", store.lastQuery)
	}
}

// TestTurnoCorrigeConElVocabularioDelModo: el vocabulario del corrector sale del MODO, por
// memory.RecallOptions.AlcanceDelCorrector, y no del filtro duro de las opciones del recall. En
// «aparte» ese filtro es el proyecto propio, pero el recall trae hasta loop.recall_otros_max notas de
// otro proyecto: con el vocabulario propio, el corrector daba por muerta una palabra de esas notas y
// la reescribía a una de éste. En «aislado», el propio; en «mezclado» y sin proyecto, todo el acervo.
//
// Sabotaje: el turno le pasa al corrector el filtro duro del recall, que en «aparte» es el propio.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\tconsulta, correcciones = store.CorregirConsulta(context.Background(), prompt, opts.AlcanceDelCorrector())\n"
// arnes: a="\t\tconsulta, correcciones = store.CorregirConsulta(context.Background(), prompt, memory.ProjectScope{ProjectID: opts.ProjectScope, Federate: opts.Federate})\n"
func TestTurnoCorrigeConElVocabularioDelModo(t *testing.T) {
	for _, c := range []struct {
		modo, propio string
		quiere       memory.ProjectScope
	}{
		{config.OtrosProyectosAparte, "musubi", memory.ProjectScope{}},
		{config.OtrosProyectosAislado, "musubi", memory.ProjectScope{ProjectID: "musubi"}},
		{config.OtrosProyectosMezclado, "musubi", memory.ProjectScope{}},
		{config.OtrosProyectosAparte, "", memory.ProjectScope{}},
	} {
		store := &fakeTurnStore{recall: itemDelPi(), corregir: corrigeDosTipeos}
		buildTurnRecall(store, parametrosDelTurno{sesion: "s-" + c.modo, prompt: promptConTipeos, presupuesto: 250,
			memCfg: config.Default().Memory, propio: c.propio, otrosModo: c.modo, otrosMax: 2})
		if !store.llamoAlCorrector {
			t.Fatalf("modo %s con propio %q: el turno no llamó al corrector", c.modo, c.propio)
		}
		if store.corrigioAlcance != c.quiere {
			t.Errorf("modo %s con propio %q: el corrector corrió con %+v y tenía que correr con %+v (el recall filtró %q con tope %d)",
				c.modo, c.propio, store.corrigioAlcance, c.quiere, store.lastOpts.ProjectScope, store.lastOpts.TopeOtrosProyectos)
		}
	}
}

// motorSinApuro es el motor real con el corrector sin el plazo de producción (60 ms): la prueba de
// abajo mide QUÉ corrige el turno, no si le alcanza el tiempo en una máquina cargada. Un plazo vencido
// devolvería el prompt sin corregir, y la mitad de la prueba que espera NO ver una corrección pasaría
// por el motivo equivocado.
type motorSinApuro struct{ *memory.DbEngine }

func (m motorSinApuro) CorregirConsulta(ctx context.Context, q string, a memory.ProjectScope) (string, []memory.Correccion) {
	return m.CorregirConsultaConPlazo(ctx, q, a, 5*time.Second)
}

// TestTurnoEnAparteNoCorrigeLaPalabraDeOtroProyecto: el caso de la revisión, de punta a punta con el
// motor real. Altura escribe «planilla» y Musubi «plantilla», a una letra. Desde el repo de Musubi, en
// «aparte», el turno trae las notas de Altura sobre la planilla, así que «planilla» está VIVA y no se
// toca. Con el vocabulario propio se leía «busqué «plantilla» por «planilla»» encima de las mismas
// notas de Altura. En «aislado» sí se corrige, porque ese recall no puede traer lo de Altura. Y esa
// mitad muestra además que la prueba no pasa porque el corrector no corrió.
func TestTurnoEnAparteNoCorrigeLaPalabraDeOtroProyecto(t *testing.T) {
	eng, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	for _, n := range []struct{ proyecto, id, topic, texto string }{
		{"musubi", "p-pla1", "sdd/plantillas", "La plantilla de artefactos SDD vive en el repo."},
		{"musubi", "p-pla2", "cuerpo/lienzo", "El lienzo arranca desde una plantilla vacía."},
		{"altura", "a-pla1", "rrhh/planilla", "La planilla del turno noche se cierra los viernes."},
		{"altura", "a-pla2", "rrhh/horas", "Las horas extra se cargan en la planilla semanal."},
	} {
		if err := eng.SaveObservationTypedFrom(n.proyecto, "", n.id, n.topic, n.texto, 1, "", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	turno := func(modo string) string {
		return buildTurnRecall(motorSinApuro{eng}, parametrosDelTurno{sesion: "s-" + modo,
			prompt: "cómo se cierra la planilla del turno noche", presupuesto: 250,
			memCfg: config.Default().Memory, propio: "musubi", otrosModo: modo, otrosMax: 2})
	}

	aparte := turno(config.OtrosProyectosAparte)
	if !strings.Contains(aparte, "[id:a-pla1]") {
		t.Fatalf("precondición: en «aparte» el turno tenía que traer la nota de Altura sobre la planilla:\n%s", aparte)
	}
	if strings.Contains(aparte, transcripts.PrefijoDeCorreccion) {
		t.Errorf("en «aparte» el turno corrigió una palabra de las notas ajenas que él mismo trae:\n%s", aparte)
	}
	aislado := turno(config.OtrosProyectosAislado)
	if !strings.Contains(aislado, transcripts.PrefijoDeCorreccion+"plantilla» por «planilla»") {
		t.Errorf("en «aislado» «planilla» no está en nada que ese recall pueda traer, y el turno no la corrigió:\n%s", aislado)
	}
}

// TestTurnoAvisaLoQueCorrigio: lo que se corrigió se avisa en UN renglón propio, entre el encabezado
// y la primera viñeta, con la forma del contrato («busqué «X» por «Y»», X lo buscado e Y lo
// tipeado). Y sólo ahí: sin viñetas no hay aviso, sin correcciones tampoco, y el priming no lo lleva.
//
// Sabotaje: el aviso pierde el prefijo del contrato con el detector de eco.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\treturn transcripts.PrefijoDeCorreccion + strings.Join(partes, \", «\")\n"
// arnes: a="\treturn \"corregí «\" + strings.Join(partes, \", «\")\n"
//
// Sabotaje: el aviso pone primero lo tipeado.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\tpartes = append(partes, c.Buscado+\"» por «\"+c.Tipeado+\"»\")\n"
// arnes: a="\t\tpartes = append(partes, c.Tipeado+\"» por «\"+c.Buscado+\"»\")\n"
//
// Sabotaje: el encabezado no avisa.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif len(correcciones) > 0 {\n\t\th += \"\\n\" + lineaDeCorreccion(correcciones)\n"
// arnes: a="\tif false && len(correcciones) > 0 {\n\t\th += \"\\n\" + lineaDeCorreccion(correcciones)\n"
//
// Sabotaje: el aviso sale aunque el bloque no traiga viñetas.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif b.Len() == 0 {\n\t\tcorrecciones = nil"
// arnes: a="\tif false && b.Len() == 0 {\n\t\tcorrecciones = nil"
//
// Sabotaje: el priming avisa una corrección.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\treturn strings.TrimRight(encabezadoDeMemoria(titulo, huboMarcas, nil)+\"\\n\"+b.String(), \"\\n\")\n"
// arnes: a="\treturn strings.TrimRight(encabezadoDeMemoria(titulo, huboMarcas, []memory.Correccion{{Tipeado: \"x\", Buscado: \"y\"}})+\"\\n\"+b.String(), \"\\n\")\n"
func TestTurnoAvisaLoQueCorrigio(t *testing.T) {
	store := &fakeTurnStore{recall: itemDelPi(), corregir: corrigeDosTipeos}
	in := strings.NewReader(`{"prompt":"` + promptConTipeos + `","hook_event_name":"UserPromptSubmit"}`)
	_, ctx := hookAdditionalContext(t, turnOutput(store, defaultLoop(), pipeOff(), maOff(), config.Default().Memory, in))
	if ctx == "" {
		t.Fatal("el hook no inyectó nada: sin bloque no hay aviso que medir")
	}
	lineas := strings.Split(ctx, "\n")
	const quiere = "busqué «informacion» por «infromacion», «comando» por «comadno»"
	if len(lineas) < 3 || lineas[1] != quiere {
		t.Fatalf("la segunda línea del bloque tenía que ser el aviso\n  quiere: %q\n  bloque:\n%s", quiere, ctx)
	}
	if !strings.HasPrefix(lineas[1], transcripts.PrefijoDeCorreccion) {
		t.Errorf("el aviso no arranca con el prefijo del contrato %q: el detector de eco lo contaría", transcripts.PrefijoDeCorreccion)
	}
	if !strings.HasPrefix(lineas[2], "- ") {
		t.Errorf("después del aviso tenía que venir la primera viñeta, y vino %q", lineas[2])
	}
	lineasDeViñeta(t, ctx)

	// Sin correcciones, no hay aviso.
	sinTipeos := &fakeTurnStore{recall: itemDelPi()}
	_, ctx = hookAdditionalContext(t, turnOutput(sinTipeos, defaultLoop(), pipeOff(), maOff(), config.Default().Memory,
		strings.NewReader(`{"prompt":"el comando del pi","hook_event_name":"UserPromptSubmit"}`)))
	if strings.Contains(ctx, transcripts.PrefijoDeCorreccion) {
		t.Errorf("sin correcciones el bloque no puede traer el aviso:\n%s", ctx)
	}

	// Sin viñetas, tampoco: el aviso presenta una lista que no está.
	_, cs := corrigeDosTipeos("")
	if b := formatDeltaGists("[Musubi — memoria relevante] Contexto.", nil, nil, "", cs); strings.Contains(b, transcripts.PrefijoDeCorreccion) {
		t.Errorf("un bloque sin viñetas trajo el aviso:\n%s", b)
	}

	// Y el priming de arranque no corrige ni avisa.
	priming := buildPrimingContext(memory.AlcanceDelTurno{}, &fakeStore{meta: map[string]string{}, prime: itemDelPi()}, 250, "s1", "")
	if priming == "" {
		t.Fatal("el priming no produjo bloque: sin bloque no hay nada que mirar")
	}
	if strings.Contains(priming, transcripts.PrefijoDeCorreccion) {
		t.Errorf("el priming trajo el aviso del corrector, que es sólo del turno:\n%s", priming)
	}
}

// TestTurnoSinCorrectorSaleComoAntes: con memory.recall_typo_correction apagado, el turno no llama
// al corrector, busca el prompt como vino y el bloque sale byte a byte igual que el de un turno en
// el que el corrector no tiene nada que corregir, que es el bloque de siempre.
//
// Sabotaje: el turno corrige aunque la perilla esté apagada.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif p.memCfg.RecallTypoCorrection {\n"
// arnes: a="\tif true || p.memCfg.RecallTypoCorrection {\n"
func TestTurnoSinCorrectorSaleComoAntes(t *testing.T) {
	apagado := config.Default().Memory
	apagado.RecallTypoCorrection = false
	entrada := `{"prompt":"` + promptConTipeos + `","hook_event_name":"UserPromptSubmit"}`

	store := &fakeTurnStore{recall: itemDelPi(), corregir: corrigeDosTipeos}
	salida := turnOutput(store, defaultLoop(), pipeOff(), maOff(), apagado, strings.NewReader(entrada))
	if store.llamoAlCorrector {
		t.Error("con la perilla apagada el turno llamó al corrector")
	}
	if store.lastQuery != promptConTipeos {
		t.Errorf("con la perilla apagada el recall tenía que buscar el prompt como vino, y buscó %q", store.lastQuery)
	}

	// La referencia es el mismo turno contra un store cuyo corrector no cambia nada.
	referencia := turnOutput(&fakeTurnStore{recall: itemDelPi()}, defaultLoop(), pipeOff(), maOff(), apagado, strings.NewReader(entrada))
	if salida == "" || salida != referencia {
		t.Errorf("con la perilla apagada la salida del hook cambió\n  salida:     %q\n  referencia: %q", salida, referencia)
	}
	// Y la forma es la de main: el encabezado en UN renglón que cierra presentando la lista, y la
	// primera viñeta enseguida, sin nada en el medio.
	_, ctx := hookAdditionalContext(t, salida)
	lineas := strings.Split(ctx, "\n")
	if len(lineas) < 2 || !strings.HasSuffix(lineas[0], " (gists; expandí con musubi_memory_expand):") || !strings.HasPrefix(lineas[1], "- ") {
		t.Errorf("con la perilla apagada el bloque no tiene la forma de siempre (encabezado y viñetas):\n%s", ctx)
	}
}
