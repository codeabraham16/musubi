package mcp

// fleet_barrido_techo_test.go custodia el TERCER techo del track, que no es del exportador: el
// que acota cuántos tenants se BARREN por tick.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// UN TECHO DE EXPORTACIÓN ACOTANDO QUÉ MÁQUINAS SE VIGILAN
//
// La sonda (`barrerFlotaUnaVez`) y el barrido de vida de red usaban literalmente
// `proyectosParaExportar`, la constante que existe para acotar la CARDINALIDAD de un scrape de
// Prometheus. Son dos preguntas sin nada que ver, y compartir la constante tenía una consecuencia
// que nadie escribió: bajar el techo del export apagaba en silencio la vigilancia y el auto-heal
// de los tenants que quedaran afuera. Sus máquinas no quedaban «vigiladas en amarillo»: quedaban
// SIN VIGILAR Y EN VERDE, que es el estado que este track existe para no tener.
//
// Y el recorte era mudo en los dos barridos: ni serie, ni log, ni perilla.
//
// LO QUE ESTA GUARDA SOSTIENE: que el techo del barrido sea SUYO (moverlo del lado del export ya
// no mueve esto) y que cuando corte, hable. Lo que NO sostiene, y queda anotado: el número sigue
// siendo 64 y no hay serie en Prometheus para este recorte — subir el fan-out de un barrido que
// abre conexiones a máquinas reales es una decisión de operación, no de una ronda de guardas.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/logx"
)

// EL TECHO DEL BARRIDO NO ES EL DEL EXPORT, y eso hoy no se puede medir por el valor: los dos
// valen 64. Lo que decide es CUÁL SÍMBOLO se escribe en el barrido, así que eso es lo que se
// mira — y se mira con el parser de Go, no con un grep: un `proyectosParaExportar` escrito en un
// comentario (hay dos, explicando justamente esto) no puede ni satisfacer ni romper la guarda.
//
// Sabotaje que la pone roja: volver a poner `proyectosParaExportar` en cualquiera de los dos
// barridos, o definir `const proyectosParaVigilar = proyectosParaExportar`.
func TestElBarridoDeLaFlotaNoCuelgaDelTechoDelExport(t *testing.T) {
	fset := token.NewFileSet()
	archivo, err := parser.ParseFile(fset, "scheduler_flota.go", nil, 0) // sin comentarios
	if err != nil {
		t.Fatalf("no se pudo parsear scheduler_flota.go: %v", err)
	}
	ast.Inspect(archivo, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if ok && id.Name == "proyectosParaExportar" {
			t.Errorf("%s: el barrido de la flota usa `proyectosParaExportar`. Ese número existe para acotar la CARDINALIDAD de un scrape de Prometheus; acá decide QUÉ MÁQUINAS SE VIGILAN Y SE REPARAN. Compartirlo hace que bajar el techo del export apague la vigilancia de los tenants que queden afuera, sin serie y sin aviso. Usá `proyectosParaVigilar`.",
				fset.Position(id.Pos()))
		}
		return true
	})

	if proyectosParaVigilar <= 0 {
		t.Fatalf("el techo del barrido es %d: con <= 0 no se vigila ni un tenant", proyectosParaVigilar)
	}
}

// CUANDO EL BARRIDO RECORTA, LO DICE. Un recorte mudo deja tenants sin sondear y sin auto-heal, y
// desde afuera eso es indistinguible de que estén todos bien.
//
// Sabotaje que la pone roja: borrar el avisoMientras de proyectosAVigilar, o devolver la lista
// entera sin marcar el recorte.
func TestElBarridoDeLaFlotaAvisaCuandoDejaTenantsAfueraYSeRearma(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	// Justo en el techo: todavía no recorta.
	for i := 0; i < proyectosParaVigilar; i++ {
		maquinaConMuestra(t, s, fmt.Sprintf("tenant-%03d", i), "server", *muestraDePrueba(), ahora)
	}

	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	proyectos, err := s.proyectosAVigilar("sonda")
	restaurar()
	if err != nil {
		t.Fatal(err)
	}
	if len(proyectos) != proyectosParaVigilar {
		t.Fatalf("con %d tenants el barrido devolvió %d", proyectosParaVigilar, len(proyectos))
	}
	if log.Len() != 0 {
		t.Errorf("no se recortó nada y el barrido avisó igual; un aviso que sale siempre no lo lee nadie:\n%s", log.String())
	}

	// UNO MÁS: ahora recorta, y ese tenant no se sondea ni se repara.
	maquinaConMuestra(t, s, "tenant-de-mas", "server", *muestraDePrueba(), ahora)
	log.Reset()
	restaurar = logx.Capturar(&log)
	proyectos, err = s.proyectosAVigilar("sonda")
	restaurar()
	if err != nil {
		t.Fatal(err)
	}
	if len(proyectos) != proyectosParaVigilar {
		t.Fatalf("el barrido devolvió %d tenants y su techo es %d", len(proyectos), proyectosParaVigilar)
	}
	if log.Len() == 0 {
		t.Fatal("el barrido dejó un tenant afuera y no dijo nada: sus máquinas no se vigilan ni se reparan, y desde afuera se ve igual que todo bien")
	}
	if !strings.Contains(log.String(), "NO se vigilan") {
		t.Errorf("el aviso del recorte no dice la consecuencia (que esos tenants dejan de vigilarse):\n%s", log.String())
	}
	// Y NO NOMBRA LA PERILLA DEL EXPORT, que no arregla nada acá: mandar a alguien a
	// `services_per_project_export` cuando lo que falta es vigilancia es el mismo defecto que le
	// dio nombre al cabo de los techos.
	if strings.Contains(log.String(), "services_per_project_export") {
		t.Errorf("el aviso del barrido ofrece la perilla del EXPORT, que no cambia qué máquinas se vigilan:\n%s", log.String())
	}

	// EL REARME: se resuelve y el aviso vuelve a estar disponible para el próximo episodio.
	if ok, err := s.engine.RevocarDevice("tenant-de-mas", "server"); err != nil || !ok {
		t.Fatalf("revocar: ok=%v err=%v", ok, err)
	}
	if _, err := s.proyectosAVigilar("sonda"); err != nil {
		t.Fatal(err)
	}
	if _, dado := s.avisosDados.Load("barrido_truncado:sonda"); dado {
		t.Error("el recorte se resolvió y el aviso quedó marcado: el próximo episodio pasa en silencio")
	}
}

// LOS DOS BARRIDOS SON HERMANOS Y CADA UNO TIENE SU PROPIA CLAVE DE AVISO. Con una sola clave, el
// primero en recortar dejaba mudo al segundo — es el mismo defecto que la ronda 1 arregló en los
// avisos del empuje, un archivo más allá.
func TestCadaBarridoTieneSuPropioAviso(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ahora := time.Now()
	for i := 0; i <= proyectosParaVigilar; i++ {
		maquinaConMuestra(t, s, fmt.Sprintf("tenant-%03d", i), "server", *muestraDePrueba(), ahora)
	}
	for _, barrido := range []string{"sonda", "vida-de-red"} {
		if _, err := s.proyectosAVigilar(barrido); err != nil {
			t.Fatal(err)
		}
		if _, dado := s.avisosDados.Load("barrido_truncado:" + barrido); !dado {
			t.Errorf("el barrido %q recortó y no dejó su propio aviso: con una clave compartida, el primero en cortar deja mudo al otro", barrido)
		}
	}
}
