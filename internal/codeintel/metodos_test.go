package codeintel

import "testing"

// Tests de las llamadas a MÉTODOS sobre el propio receptor (Track 20 · F8-C).
//
// El defecto que los motiva se midió: hasta acá NINGÚN método era destino de una llamada. Sobre el
// código real de Musubi, 4.115 aristas CALLS y CERO llegando a un método. Consecuencia: un
// envoltorio que sólo delega en `s.Interno()` no mostraba a quién le pega cambiar `Interno`, y en un
// paquete como `internal/mcp` —donde casi todo es `McpServer.*`— eso deja a `impact` casi ciego.
//
// La clave del alcance: resolver `x.Metodo()` EN GENERAL exige inferencia de tipos. Pero adentro de
// un método, el tipo del receptor está DECLARADO EN SU PROPIA FIRMA, así que `s.Otro()` se resuelve
// leyendo el AST y nada más. Eso es lo que entra. Todo lo demás sigue afuera, y varios de estos
// tests existen para fijarlo.

func calleesDe(g PackageGraph, from string) map[string]bool {
	out := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == EdgeCalls && e.FromKey == from {
			out[e.ToKey] = true
		}
	}
	return out
}

// EL CASO CENTRAL: un método llama a otro método del mismo tipo por su receptor.
func TestMetodo_LlamaAHermanoPorSuReceptor(t *testing.T) {
	files := map[string]string{
		"pkg/a.go": `package pkg

type Servidor struct{}

func (s *Servidor) Envoltorio() {
	s.Interno()
}

func (s *Servidor) Interno() {}
`,
	}
	g := DerivePackage("pkg", files, modPath)

	c := calleesDe(g, "pkg/a.go#method:Servidor.Envoltorio")
	if !c["pkg/a.go#method:Servidor.Interno"] {
		t.Errorf("el envoltorio no llegó al método interno; callees = %v", c)
	}
}

// Cruza ARCHIVOS del mismo paquete: los métodos de un tipo suelen estar repartidos, y la tabla del
// paquete se arma con todos los archivos antes de resolver.
func TestMetodo_CruzaArchivosDelMismoPaquete(t *testing.T) {
	files := map[string]string{
		"pkg/a.go": "package pkg\n\ntype S struct{}\n\nfunc (s *S) Uno() { s.Dos() }\n",
		"pkg/b.go": "package pkg\n\nfunc (s *S) Dos() {}\n",
	}
	g := DerivePackage("pkg", files, modPath)

	if !calleesDe(g, "pkg/a.go#method:S.Uno")["pkg/b.go#method:S.Dos"] {
		t.Error("no resolvió el método hermano declarado en otro archivo del paquete")
	}
}

// ⚠️ EL ORDEN IMPORTA: dentro del método, el receptor SOMBREA a un import homónimo. Si se mirara
// el import primero, la llamada se le adjudicaría al paquete equivocado — una arista INVENTADA, que
// es peor que una ausente.
func TestMetodo_ElReceptorLeGanaAlImportHomonimo(t *testing.T) {
	files := map[string]string{
		"pkg/a.go": `package pkg

import "example.com/mod/internal/util"

type T struct{}

// El receptor se llama util, igual que el import. Adentro, util es el receptor.
func (util *T) Hace() {
	util.Ayuda()
}

func (t *T) Ayuda() {}

func Otra() { util.Ayuda() }
`,
	}
	g := DerivePackage("pkg", files, modPath)

	if !calleesDe(g, "pkg/a.go#method:T.Hace")["pkg/a.go#method:T.Ayuda"] {
		t.Error("adentro del método, `util` es el RECEPTOR: la llamada es al método hermano")
	}
	for _, pc := range g.PendingCalls {
		if pc.FromKey == "pkg/a.go#method:T.Hace" {
			t.Errorf("emitió un pendiente cross-paquete desde el método: el import le ganó al receptor (%+v)", pc)
		}
	}
	// Control: en una FUNC sin receptor, el mismo `util.Ayuda()` sí es el import.
	hay := false
	for _, pc := range g.PendingCalls {
		if pc.FromKey == "pkg/a.go#func:Otra" && pc.ImportPath == modPath+"/internal/util" {
			hay = true
		}
	}
	if !hay {
		t.Error("fuera del método no hay receptor que sombree: `util.Ayuda()` tiene que ser el import")
	}
}

// FUERA DE ALCANCE, y se fija a propósito: llamar a un método a través de un CAMPO o de otra
// variable exige saber su tipo, o sea inferencia. Resolverlo a ojo inventaría aristas.
func TestMetodo_LoQueNecesitaInferirTiposNoSeResuelve(t *testing.T) {
	files := map[string]string{
		"pkg/a.go": `package pkg

type Otro struct{}

func (o *Otro) Hace() {}

type S struct{ campo *Otro }

func (s *S) PorCampo()    { s.campo.Hace() }
func (s *S) PorVariable() { var v Otro; v.Hace() }
func (s *S) Propio()      { s.Bien() }
func (s *S) Bien()        {}
`,
	}
	g := DerivePackage("pkg", files, modPath)

	if c := calleesDe(g, "pkg/a.go#method:S.PorCampo"); len(c) != 0 {
		t.Errorf("`s.campo.Hace()` se resolvió sin poder saber el tipo del campo: %v", c)
	}
	if c := calleesDe(g, "pkg/a.go#method:S.PorVariable"); len(c) != 0 {
		t.Errorf("`v.Hace()` se resolvió sin inferir el tipo de la variable: %v", c)
	}
	// Y el control que le da sentido a los dos de arriba: el caso que SÍ está en alcance funciona.
	if !calleesDe(g, "pkg/a.go#method:S.Propio")["pkg/a.go#method:S.Bien"] {
		t.Error("la llamada por el propio receptor tiene que resolver: si no, los otros dos pasan por el motivo equivocado")
	}
}

// Un método puede declarar el receptor sin nombrarlo, o descartarlo con `_`. No hay por dónde
// llamar nada, así que no se recolecta — sin pánico y sin inventar.
func TestMetodo_ReceptorSinNombreODescartado(t *testing.T) {
	files := map[string]string{
		"pkg/a.go": `package pkg

type S struct{}

func (*S) SinNombre()  { Libre() }
func (_ *S) Descartado() { Libre() }
func (s *S) Normal()   { s.Objetivo() }
func (s *S) Objetivo() {}
func Libre()           {}
`,
	}
	g := DerivePackage("pkg", files, modPath)

	// No revientan y siguen viendo la func top-level, que se llama sin calificar.
	if !calleesDe(g, "pkg/a.go#method:S.SinNombre")["pkg/a.go#func:Libre"] {
		t.Error("un receptor sin nombre no debe impedir resolver las llamadas SIN calificar")
	}
	if !calleesDe(g, "pkg/a.go#method:S.Descartado")["pkg/a.go#func:Libre"] {
		t.Error("un receptor `_` no debe impedir resolver las llamadas sin calificar")
	}
	if !calleesDe(g, "pkg/a.go#method:S.Normal")["pkg/a.go#method:S.Objetivo"] {
		t.Error("el receptor con nombre sí resuelve")
	}
}

// Dos tipos con un método homónimo: cada uno tiene que ir al SUYO. La tabla se indexa por
// "Receptor.Metodo", no por nombre pelado — si fuera por nombre, uno pisaría al otro y la mitad de
// las aristas apuntaría al tipo equivocado.
func TestMetodo_HomonimosDeTiposDistintosNoSeMezclan(t *testing.T) {
	files := map[string]string{
		"pkg/a.go": `package pkg

type A struct{}
type B struct{}

func (a *A) Llama() { a.Comun() }
func (b *B) Llama() { b.Comun() }
func (a *A) Comun() {}
func (b *B) Comun() {}
`,
	}
	g := DerivePackage("pkg", files, modPath)

	if !calleesDe(g, "pkg/a.go#method:A.Llama")["pkg/a.go#method:A.Comun"] {
		t.Error("A.Llama tiene que ir a A.Comun")
	}
	if calleesDe(g, "pkg/a.go#method:A.Llama")["pkg/a.go#method:B.Comun"] {
		t.Error("A.Llama se fue al Comun de B: la tabla está indexada por nombre pelado")
	}
	if !calleesDe(g, "pkg/a.go#method:B.Llama")["pkg/a.go#method:B.Comun"] {
		t.Error("B.Llama tiene que ir a B.Comun")
	}
}

// Un método que llama a algo que NO existe en el tipo (por ejemplo, promovido de un embebido) se
// OMITE. Resolver embebidos exige seguir la cadena de tipos: fuera de alcance.
func TestMetodo_LoQueNoEstaEnElTipoSeOmite(t *testing.T) {
	files := map[string]string{
		"pkg/a.go": `package pkg

type Base struct{}

func (b *Base) Heredado() {}

type S struct{ Base }

func (s *S) Usa() { s.Heredado() }
`,
	}
	g := DerivePackage("pkg", files, modPath)

	if c := calleesDe(g, "pkg/a.go#method:S.Usa"); len(c) != 0 {
		t.Errorf("resolvió un método PROMOVIDO por embebido sin seguir la cadena de tipos: %v", c)
	}
}
