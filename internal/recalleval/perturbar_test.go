package recalleval

import (
	"context"
	"os"
	"testing"

	"musubi/internal/embedding"
)

// Pruebas de la perturbación por clase de tipeo (perturbar.go): que el instrumento meta el error
// que dice, que mueva el dorado, y el banco de tipeo sobre una memoria real con el ranker del turno.

// TestPerturbarConsultaPorClase: cada clase mete exactamente el error que declara, en CADA término
// de 5 runas o más que sea todo letras, sin tocar la primera letra ni lo que no es término; es
// determinista; y el brazo de un tipeo toca sólo el término más largo.
//
// CON 64 SEMILLAS Y NO CON UNA. Con una sola (la 7) ningún término caía en la posición 0, así que
// una perturbación que podía sacar la primera letra seguía verde: 64 semillas por seis términos son
// 384 errores por clase, y la posición 0 no se escapa.
//
// Sabotaje: los términos de exactamente 5 runas dejan de recibir tipeo.
// arnes: archivo="internal/recalleval/perturbar.go"
// arnes: de="if len(palabra) < minRunasPerturbables {"
// arnes: a="if len(palabra) <= minRunasPerturbables {"
//
// Sabotaje: «falta» puede sacar la primera letra.
// arnes: archivo="internal/recalleval/perturbar.go"
// arnes: de="\tcase TipeoFalta:\n\t\tk := 1 + int(h%uint64(n-1))"
// arnes: a="\tcase TipeoFalta:\n\t\tk := int(h % uint64(n))"
func TestPerturbarConsultaPorClase(t *testing.T) {
	// «quedó» tiene 5 runas justas (y una tilde: se cuenta en runas, no en bytes); «deploy2prod» no
	// es todo letras; «el», «del», «en», «y», «la» son cortos.
	const q = "el fichaje del kiosko quedó en blanco, revisá deploy2prod y la raspberry"
	quierenTipeo := map[string]bool{"fichaje": true, "kiosko": true, "quedó": true, "blanco": true, "revisá": true, "raspberry": true}

	orig := terminosComoTexto(q)
	for _, clase := range ClasesDeTipeo {
		// Dos llamadas separadas: comparar la expresión consigo misma la marca staticcheck (SA4000).
		primera, segunda := PerturbarConsulta(q, clase, 7), PerturbarConsulta(q, clase, 7)
		if primera != segunda {
			t.Errorf("%s: la misma entrada dio dos salidas: no es determinista", clase)
		}
		if PerturbarConsulta(q, clase, 7) == PerturbarConsulta(q, clase, 8) {
			t.Errorf("%s: dos semillas dieron la misma perturbación", clase)
		}
		for semilla := uint64(1); semilla <= 64; semilla++ {
			p := PerturbarConsulta(q, clase, semilla)
			pert := terminosComoTexto(p)
			if len(pert) != len(orig) {
				t.Fatalf("%s/%d: %q tiene %d términos y el original %d: el tipeo partió o fundió términos", clase, semilla, p, len(pert), len(orig))
			}
			if separadores(p) != separadores(q) {
				t.Errorf("%s/%d: cambió lo que no es término: %q", clase, semilla, p)
			}
			for i, o := range orig {
				n := pert[i]
				if !quierenTipeo[o] {
					if n != o {
						t.Errorf("%s/%d: %q no es perturbable y quedó %q", clase, semilla, o, n)
					}
					continue
				}
				if !esTipeoDeClase(o, n, clase) {
					t.Errorf("%s/%d: %q → %q no es un error de esa clase (o tocó la primera letra)", clase, semilla, o, n)
				}
			}
		}
	}

	if got := PerturbarConsulta(q, ClaseDeTipeo("otra"), 7); got != q {
		t.Errorf("una clase desconocida tiene que devolver q sin cambios, dio %q", got)
	}

	// UN tipeo: sólo el término más largo («raspberry»).
	uno := terminosComoTexto(PerturbarTerminoMasLargo(q, TipeoTransposicion, 7))
	for i, o := range orig {
		cambio := uno[i] != o
		if cambio != (o == "raspberry") {
			t.Errorf("PerturbarTerminoMasLargo: %q → %q (sólo tenía que cambiar «raspberry»)", o, uno[i])
		}
	}
}

// TestSobraMezclaLetraAjenaYRepetida: la letra de más es la de los tipeos del dueño. En sus prompts
// (3.407, clasificados en la revisión de este banco) la letra de más es una letra AJENA en el medio
// en la gran mayoría de los casos y una REPETIDA en ~16-18 %. La primera versión sólo repetía, que
// para el corrector es la forma fácil. Acá se exige la mezcla (repetidas entre el 10 y el 25 %; una «ajena»
// igual a su vecina cuenta como repetida) y que la ajena nunca vaya antes de la primera letra ni al
// final.
//
// Sabotaje: «sobra» vuelve a repetir siempre.
// arnes: archivo="internal/recalleval/perturbar.go"
// arnes: de="\t\tif (h>>32)%sobraCadaCuantasRepite == 0 {"
// arnes: a="\t\tif true {"
func TestSobraMezclaLetraAjenaYRepetida(t *testing.T) {
	palabras := []string{"información", "raspberry", "fichaje", "kiosko", "memoria", "terminal"}
	repetidas, ajenas := 0, 0
	for _, pal := range palabras {
		w := []rune(pal)
		for semilla := uint64(1); semilla <= 300; semilla++ {
			b := []rune(PerturbarConsulta(pal, TipeoSobra, semilla))
			if len(b) != len(w)+1 {
				t.Fatalf("%q → %q: «sobra» tiene que agregar exactamente una letra", pal, string(b))
			}
			// j: dónde entró la letra de más (la primera posición cuyo borrado devuelve la palabra).
			j := -1
			for k := range b {
				if string(b[:k])+string(b[k+1:]) == pal {
					j = k
					break
				}
			}
			if j < 0 {
				t.Fatalf("%q → %q: sacando una letra no vuelve la palabra", pal, string(b))
			}
			// Repetida: la letra de más es igual a la que le sigue (j es la primera de las dos).
			if j+1 < len(b) && b[j] == b[j+1] {
				repetidas++
				continue
			}
			ajenas++
			if j == 0 || j == len(b)-1 {
				t.Errorf("%q → %q: la letra ajena entró en la punta (posición %d)", pal, string(b), j)
			}
		}
	}
	total := repetidas + ajenas
	frac := float64(repetidas) / float64(total)
	t.Logf("sobra: %d repetidas y %d ajenas de %d (%.1f %% repetidas)", repetidas, ajenas, total, 100*frac)
	if frac < 0.10 || frac > 0.25 {
		t.Errorf("sobra: %.1f %% de letras repetidas; los tipeos del dueño tienen ~16-18 %% (se acepta 10-25 %%)", 100*frac)
	}
}

// terminosComoTexto devuelve los términos de q con el mismo corte que usa la perturbación.
func terminosComoTexto(q string) []string {
	r := []rune(q)
	var out []string
	for _, t := range terminosDe(r) {
		out = append(out, string(r[t.ini:t.fin]))
	}
	return out
}

// separadores devuelve q sin sus términos: lo que la perturbación no puede tocar.
func separadores(q string) string {
	r := []rune(q)
	var out []rune
	copiado := 0
	for _, t := range terminosDe(r) {
		out = append(out, r[copiado:t.ini]...)
		out = append(out, '|')
		copiado = t.fin
	}
	return string(append(out, r[copiado:]...))
}

// esTipeoDeClase dice si n sale de o con UN error de la clase, sin tocar la primera runa.
func esTipeoDeClase(o, n string, clase ClaseDeTipeo) bool {
	a, b := []rune(o), []rune(n)
	if len(b) == 0 || a[0] != b[0] {
		return false
	}
	switch clase {
	case TipeoTransposicion:
		if len(a) != len(b) {
			return false
		}
		var dif []int
		for i := range a {
			if a[i] != b[i] {
				dif = append(dif, i)
			}
		}
		return len(dif) == 2 && dif[1] == dif[0]+1 && a[dif[0]] == b[dif[1]] && a[dif[1]] == b[dif[0]]
	case TipeoFalta:
		return len(b) == len(a)-1 && sacandoUnaQueda(a, b)
	case TipeoSobra:
		return len(b) == len(a)+1 && sacandoUnaQueda(b, a)
	case TipeoSustitucion:
		if len(a) != len(b) {
			return false
		}
		dif := 0
		for i := range a {
			if a[i] != b[i] {
				dif++
			}
		}
		return dif == 1
	}
	return false
}

// sacandoUnaQueda dice si sacándole a largo UNA runa (que no sea la primera) queda corto.
func sacandoUnaQueda(largo, corto []rune) bool {
	for k := 1; k < len(largo); k++ {
		if string(largo[:k])+string(largo[k+1:]) == string(corto) {
			return true
		}
	}
	return false
}

// perturbarFixture devuelve una copia de fx con cada consulta pasada por f, y cuántas cambiaron.
// Los docs y las etiquetas son los mismos: sólo cambia lo que tipea la persona.
func perturbarFixture(fx *Fixture, f func(string) string) (*Fixture, int) {
	p := *fx
	p.Queries = make([]Query, len(fx.Queries))
	cambiadas := 0
	for i, q := range fx.Queries {
		q.Text = f(q.Text)
		if q.Text != fx.Queries[i].Text {
			cambiadas++
		}
		p.Queries[i] = q
	}
	return &p, cambiadas
}

// TestLaPerturbacionMueveElDorado es la cordura del INSTRUMENTO: sobre golden.json, con el ranker
// del turno, tipear todos los términos de 5 runas o más baja el MRR 0,10 o más en CADA clase. Si el
// instrumento no mueve el dorado, un gate construido encima (el del corrector) no puede ponerse
// rojo por nada.
//
// Por qué el promedio de tres semillas y no una: medido al escribir esta prueba, con la semilla 1
// la clase «falta» baja sólo 0,055 (0,722 → 0,667), porque sacar la ÚLTIMA letra deja un prefijo de
// la palabra y el match por prefijo de raíz lo encuentra igual («fusion» → «fusio», «cells» →
// «cell»). Con las semillas 2 y 3 baja 0,222 y 0,139. Es un efecto real (el recall ya tolera esa
// forma de tipeo) y no un defecto del instrumento, pero una sola semilla lo vuelve lotería.
// Promedio de las semillas 1-3 al escribirla: transposición 0,500 · falta 0,583 · sobra 0,500 ·
// sustitución 0,500, contra 0,722 limpio.
//
// Sabotaje: PerturbarConsulta devuelve la consulta sin tocar.
// arnes: archivo="internal/recalleval/perturbar.go"
// arnes: de="\treturn perturbar(q, clase, semilla, false)"
// arnes: a="\treturn q"
//
// Sabotaje: la perturbación vuelve a tipear sólo el término más largo (las consultas cambian, pero
// el MRR del dorado casi no se mueve: es el instrumento que el crítico del plan refutó).
// arnes: archivo="internal/recalleval/perturbar.go"
// arnes: de="\t\tcase !soloElMasLargo:\n\t\t\taTipear = append(aTipear, t)"
// arnes: a="\t\tcase false:\n\t\t\taTipear = append(aTipear, t)"
func TestLaPerturbacionMueveElDorado(t *testing.T) {
	fx := loadGolden(t)
	ctx := context.Background()
	eng, err := SeedEngine(t.TempDir(), fx, nil)
	if err != nil {
		t.Fatalf("SeedEngine: %v", err)
	}
	defer eng.Close()
	ks := []int{10}
	cfg := ConfigTurno()

	limpio, err := Evaluate(ctx, eng, fx, cfg, nil, ks)
	if err != nil {
		t.Fatal(err)
	}
	const semillas = 3
	for _, clase := range ClasesDeTipeo {
		var suma float64
		for s := uint64(1); s <= semillas; s++ {
			pfx, cambiadas := perturbarFixture(fx, func(q string) string { return PerturbarConsulta(q, clase, s) })
			if cambiadas == 0 {
				t.Errorf("%s/semilla %d: ninguna consulta cambió", clase, s)
			}
			sc, err := Evaluate(ctx, eng, pfx, cfg, nil, ks)
			if err != nil {
				t.Fatal(err)
			}
			suma += sc.MRR
		}
		media := suma / semillas
		t.Logf("%-13s MRR %.3f → %.3f (−%.3f)", clase, limpio.MRR, media, limpio.MRR-media)
		if limpio.MRR-media < 0.10 {
			t.Errorf("%s: tipear todos los términos largos bajó el MRR del dorado sólo %.3f (%.3f → %.3f); "+
				"un instrumento que no mueve el dorado no puede defender un gate", clase, limpio.MRR-media, limpio.MRR, media)
		}
	}
}

// TestTipeoFixtureReal es un INSTRUMENTO, no un gate: mide cuánto pierde el ranker del hook cuando
// la persona tipea mal, sobre una COPIA de una memoria real. Es la línea base del frente búsqueda de
// la ola 2: el corrector de tipeo (ola2/corrector-de-tipeo) y el vector del turno se miden contra
// estos números.
//
// Todos los brazos van en el POOL DEL TURNO, porque es lo que el hook corre, y el brazo del hook
// corre con SU MOTOR: sin embebedor, así que MMR queda inerte aunque las opciones digan 0,75 (ver
// Config.SinEmbebedor). Los otros tres brazos limpios son los que necesita ola2/vector-en-el-turno
// para decidir: léxico con MMR, híbrido con MMR (lo que da encender el embebedor tal cual, porque
// enciende las dos cosas) e híbrido sin MMR. El único brazo con pool = corpus está para medir cuánto
// le cuesta al hook su pool de 50. Los brazos de tipeo van por clase, sustitución incluida, porque
// el corrector va a arreglar tres clases y no la cuarta: medir sólo transposiciones sería medirlo
// con el error que sabe arreglar.
//
// No tiene gate a propósito, igual que TestABLexicoVsHibridoFixtureReal: la memoria de trabajo de
// alguien cambia entre corridas. Tarda ~40 minutos, casi todo en sembrar los vectores.
//
//	GOWORK=off MUSUBI_FIXTURE_DB=<COPIA de .musubi/memory.db> \
//	MUSUBI_POTION_DIR=<.musubi/embeddings/potion-multilingual-128M> \
//	go test ./internal/recalleval -run TestTipeoFixtureReal -count=1 -v -timeout 90m
func TestTipeoFixtureReal(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirPotion := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirPotion == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB y/o MUSUBI_POTION_DIR: se saltea el banco de tipeo sobre memoria real")
	}
	prov, err := embedding.NewStaticProvider(dirPotion)
	if err != nil {
		t.Fatalf("NewStaticProvider(%s): %v", dirPotion, err)
	}
	embed := func(texto string) ([]float32, error) { return prov.Embed(context.Background(), texto) }

	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB(%s): %v", ruta, err)
	}
	t.Logf("corpus real: %d docs · %d consultas · embedder %s", len(fx.Docs), len(fx.Queries), prov.Name())
	eng, err := SeedEngine(t.TempDir(), fx, embed)
	if err != nil {
		t.Fatalf("SeedEngine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()
	ks := []int{1, 5, 10}
	hook := ConfigTurno() // léxico, pool 50, sin embebedor: MMR inerte
	lexicoMMR := ConfigTurno()
	lexicoMMR.SinEmbebedor = false // el motor ve los vectores: MMR 0,75 corre
	hibridoMMR := ConfigTurnoHibrido()
	hibridoSinMMR := ConfigTurnoHibrido()
	hibridoSinMMR.Opts.MMRLambda = 1
	poolCorpus := ConfigTurno()
	poolCorpus.PoolDelTurno = false

	type brazo struct {
		nombre string
		cfg    Config
		fx     *Fixture
	}
	brazos := []brazo{
		{"hook-lexico-pool50-sinMMR", hook, fx},
		{"lexico-pool50-MMR", lexicoMMR, fx},
		{"hibrido-pool50-MMR", hibridoMMR, fx},
		{"hibrido-pool50-sinMMR", hibridoSinMMR, fx},
		{"lexico-poolCorpus-sinMMR", poolCorpus, fx},
	}
	const semilla = 1
	for _, clase := range ClasesDeTipeo {
		uno, nUno := perturbarFixture(fx, func(q string) string { return PerturbarTerminoMasLargo(q, clase, semilla) })
		todos, nTodos := perturbarFixture(fx, func(q string) string { return PerturbarConsulta(q, clase, semilla) })
		t.Logf("%-13s consultas cambiadas: un tipeo %d · todos %d (de %d)", clase, nUno, nTodos, len(fx.Queries))
		brazos = append(brazos,
			brazo{"hook-un-tipeo-" + string(clase), hook, uno},
			brazo{"hook-todos-" + string(clase), hook, todos},
			brazo{"hibMMR-un-tipeo-" + string(clase), hibridoMMR, uno},
			brazo{"hibMMR-todos-" + string(clase), hibridoMMR, todos},
			brazo{"hibSinMMR-un-tipeo-" + string(clase), hibridoSinMMR, uno},
			brazo{"hibSinMMR-todos-" + string(clase), hibridoSinMMR, todos},
		)
	}

	var scores []Scores
	for _, b := range brazos {
		cfg := b.cfg
		cfg.Name = b.nombre
		s, err := Evaluate(ctx, eng, b.fx, cfg, embed, ks)
		if err != nil {
			t.Fatalf("%s: %v", b.nombre, err)
		}
		scores = append(scores, s)
	}
	t.Logf("\n%s", FormatReport(scores, ks))
	for _, s := range scores {
		t.Logf("  %-34s MRR %.3f · R@10 %.3f", s.Config, s.MRR, s.RecallAtK[10])
	}
	t.Log("OJO: los ABSOLUTOS están subestimados (las etiquetas salen del topic_key). Lo comparable es el DELTA entre brazos.")
}
