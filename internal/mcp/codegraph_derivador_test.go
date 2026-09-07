package mcp

import (
	"context"
	"testing"

	"musubi/internal/codeintel"
	"musubi/internal/memory"
)

// EL GRAFO NO SABÍA QUE EL DERIVADOR HABÍA CAMBIADO, y por eso quedaron regiones derivadas por un
// motor viejo durante tres semanas sin que nada lo declarara.
//
// El caso real: el resolvedor cross-paquete nació el 2026-08-14 a las 17:20 (#309) y seis paquetes
// —privacy, provision, detector, cognition, redact, skillsource— tenían sus filas escritas ESE MISMO
// DÍA a las 03:20, catorce horas antes. Sus 47 archivos no cambiaron ni un byte, así que el índice
// incremental los salteaba para siempre: el src_fingerprint es el sha256 del CONTENIDO, o sea que
// sigue al input y es ciego a quién lo derivó. Se leía como «el algoritmo falla en algunos
// paquetes» y era dato viejo.

// EL CONTROL, y no es decorativo: sin él, «re-derivar siempre» pasaría el test de abajo y la
// optimización del incremental estaría muerta sin que nadie se entere.
func TestElIncrementalSalteaCuandoNadaCambio(t *testing.T) {
	_, s := proyectoDosPaquetes(t)
	ctx := context.Background()

	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("índice completo: %v", err)
	}

	res, err := s.indexIncremental(ctx)
	if err != nil {
		t.Fatalf("índice incremental: %v", err)
	}
	if n := res["packages"].(int); n != 0 {
		t.Errorf("nada cambió y el derivador es el mismo: no debe re-derivar ningún paquete, re-derivó %d", n)
	}
	if n := res["skipped"].(int); n == 0 {
		t.Error("nada cambió: los archivos tienen que contarse como salteados")
	}
	if _, hay := res["deriver_changed"]; hay {
		t.Error("el derivador no cambió: no corresponde declarar el cambio")
	}
}

// EL TEST QUE REPRODUCE EL BUG REAL. Los archivos no cambian ni un byte —igual que en el repo— así
// que el fingerprint coincide y el incremental los saltearía. Lo único distinto es el sello del
// derivador.
func TestElIncrementalReDerivaTodoCuandoElDerivadorCambio(t *testing.T) {
	_, s := proyectoDosPaquetes(t)
	ctx := context.Background()

	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("índice completo: %v", err)
	}
	// Así queda el grafo tras un índice hecho por un motor que todavía no sabía emitir aristas
	// cross-paquete: filas presentes, fingerprints al día, sello viejo.
	if err := s.engine.SetMeta(memory.MetaCodegraphDeriver, "1-antes-del-cross-paquete"); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}

	res, err := s.indexIncremental(ctx)
	if err != nil {
		t.Fatalf("índice incremental: %v", err)
	}
	if n := res["packages"].(int); n == 0 {
		t.Fatal("el derivador cambió: hay que re-derivar aunque ningún archivo se haya tocado. " +
			"Con 0 paquetes, las filas viejas se quedan viejas para siempre.")
	}
	if res["deriver_changed"] != true {
		t.Error("el cambio de derivador tiene que declararse: si no, una barrida entera se lee como un cuelgue")
	}
	if res["deriver_to"] != codeintel.GraphDeriverVersion {
		t.Errorf("deriver_to debería ser %q, es %v", codeintel.GraphDeriverVersion, res["deriver_to"])
	}
}

// La otra mitad: después de barrer, el sello queda al día. Sin esto la puerta se traba abierta y
// CADA corrida re-deriva el repo entero — un incremental que dejó de ser incremental.
func TestElSelloDelDerivadorQuedaAlDiaTrasBarrer(t *testing.T) {
	_, s := proyectoDosPaquetes(t)
	ctx := context.Background()

	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("índice completo: %v", err)
	}
	_ = s.engine.SetMeta(memory.MetaCodegraphDeriver, "1-viejo")
	if _, err := s.indexIncremental(ctx); err != nil {
		t.Fatalf("primer incremental: %v", err)
	}

	v, ok, _ := s.engine.GetMeta(memory.MetaCodegraphDeriver)
	if !ok || v != codeintel.GraphDeriverVersion {
		t.Fatalf("tras barrer, el sello debe quedar en %q, quedó en %q (existe=%v)",
			codeintel.GraphDeriverVersion, v, ok)
	}

	// Y la segunda corrida vuelve a ser barata, que es todo el punto de un incremental.
	res, err := s.indexIncremental(ctx)
	if err != nil {
		t.Fatalf("segundo incremental: %v", err)
	}
	if n := res["packages"].(int); n != 0 {
		t.Errorf("la segunda corrida ya no debe re-derivar nada, re-derivó %d", n)
	}
}

// EL ÍNDICE COMPLETO DECLARA CUÁNTOS ENCONTRÓ, no sólo cuántos le salieron bien.
//
// Antes devolvía {"packages": N} contando únicamente los éxitos, y hacía `continue` mudo ante
// cualquier error: un full que cubría una fracción del repo reportaba éxito, y el agujero se leía
// después como «el grafo no conoce ese símbolo». Es el mismo principio que el recorte de #332.
func TestElIndiceCompletoDeclaraCuantosEncontro(t *testing.T) {
	_, s := proyectoDosPaquetes(t)

	res, err := s.indexAllPackages(context.Background())
	if err != nil {
		t.Fatalf("índice completo: %v", err)
	}
	total, hay := res["total_packages"].(int)
	if !hay {
		t.Fatal("falta total_packages: sin él, 'packages' no se puede leer como cobertura")
	}
	if total < 2 {
		t.Errorf("el proyecto tiene 2 paquetes (caller y util), total_packages dice %d", total)
	}
	if got := res["packages"].(int); got != total {
		t.Errorf("todos los paquetes son derivables: packages=%d debería igualar total_packages=%d", got, total)
	}
	// Sin fallas no se ensucia la respuesta con ruido.
	if _, hay := res["skipped"]; hay {
		t.Errorf("no falló ningún paquete: no corresponde reportar skipped (%v)", res["skipped"])
	}
}

// El recorte de motivos declara que recortó, en vez de cortar callado. Una lista truncada que no
// dice que lo está se lee como completa — que es cómo un problema de 40 directorios se ve igual
// que uno de 10.
func TestElRecorteDeMotivosSeDeclara(t *testing.T) {
	muchos := make([]string, 25)
	for i := range muchos {
		muchos[i] = "dir: error"
	}
	out := recortarMotivos(muchos, 10)
	if len(out) != 11 {
		t.Fatalf("esperaba 10 motivos + la línea del recorte, obtuve %d", len(out))
	}
	if last := out[len(out)-1]; last != "… y 15 más" {
		t.Errorf("la última entrada tiene que declarar cuántos quedaron afuera, dice %q", last)
	}

	// Y si entra entero, no se inventa una línea de recorte.
	corto := []string{"a", "b"}
	if got := recortarMotivos(corto, 10); len(got) != 2 {
		t.Errorf("una lista que entra entera no se toca, quedó en %v", got)
	}
}

// La regla del sello, afirmada directo. Hacer fallar un directorio de verdad pide permisos del
// sistema de archivos y se comporta distinto en cada plataforma, así que la decisión se prueba
// donde vive. El sabotaje lo pidió: quitarle la condición de fallidos no ponía rojo a ningún test,
// porque en el fixture nunca falla un directorio.
func TestElSelloNoAvanzaConDirectoriosFallidos(t *testing.T) {
	casos := []struct {
		cambio   bool
		fallidos int
		quiero   bool
		porque   string
	}{
		{true, 0, true, "barrida completa y sin fallas: el sello avanza"},
		{true, 1, false, "con un solo directorio fallido el sello NO puede avanzar: quedaría derivado por el motor viejo para siempre"},
		{true, 40, false, "muchos fallidos, misma regla"},
		{false, 0, false, "el derivador no cambió: no hay nada que sellar"},
		{false, 3, false, "sin cambio de derivador, los fallidos no habilitan un sello"},
	}
	for _, c := range casos {
		if got := debeSellarDerivador(c.cambio, c.fallidos); got != c.quiero {
			t.Errorf("debeSellarDerivador(cambio=%v, fallidos=%d) = %v, quería %v — %s",
				c.cambio, c.fallidos, got, c.quiero, c.porque)
		}
	}
}
