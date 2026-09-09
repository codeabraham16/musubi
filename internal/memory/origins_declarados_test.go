package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contenidoSQL es un archivo que el extractor NO entiende. Medido: ExtractSymbols sobre un `.sql`
// devuelve CERO símbolos, y lo mismo para .rs, .java, .php y .rb. Es el caso real y el más grande:
// Altura-erp tiene 398 archivos `.sql` con 612 `create or replace function` que ningún camino del
// sistema ve, y una nota sobre cualquiera de ellas no se podía anclar.
//
// Se eligió `.sql` a propósito y no `.jsx`: medí que las tres regex de lenguajes de llaves SÍ
// levantan `function`, `const X = () =>` y `class` de un .jsx (no así los métodos de una clase).
// Probar el fallback contra un archivo que el extractor entiende habría dado una prueba verde que
// no ejercita nada — el extractor gana por precedencia y el fallback nunca correría.
const contenidoSQL = `-- migración de alertas
create table alertas (
  id serial primary key
);

create or replace function evaluar_alertas() returns void as $$
begin
  perform 1;
end;
$$ language plpgsql;

create or replace function limpiar_alertas() returns void as $$
begin
  delete from alertas;
end;
$$ language plpgsql;
`

// declararGist guarda el gist de un archivo con su línea de símbolos, que es de donde sale el
// fallback. Es lo que ya hace save_code sin que nadie se lo pida.
func declararGist(t *testing.T, e *DbEngine, path, symbols string) {
	t.Helper()
	if err := e.SaveCodeMemory(CodeMemory{Path: path, Gist: "un gist", Symbols: symbols}); err != nil {
		t.Fatalf("SaveCodeMemory(%s): %v", path, err)
	}
}

// EL CASO QUE ANTES ERA UN ERROR: anclar a un símbolo de un archivo sin extractor.
//
// Es el hueco entero de la Fase 1. En Altura-erp hay 529 observaciones y CERO anclas, y no es
// porque nadie quisiera anclarlas: es porque `saveObservationOrigins` devolvía «no tiene un
// símbolo top-level llamado X» para todo lo que no fuera Go. La memoria no se vencía nunca ahí.
//
// Sabotaje: sacar el fallback a declarados de symbolFingerprintCon → vuelve el error.
func TestSePuedeAnclarAUnSimboloDeclaradoEnUnArchivoSinExtractor(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"supabase/migrations/alertas.sql": contenidoSQL})
	declararGist(t, engine, "supabase/migrations/alertas.sql", "create table alertas L2; evaluar_alertas L6-11; limpiar_alertas L13-18")

	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "el kiosko entra por landingForRole", 1.0, "", "local",
		[]string{"supabase/migrations/alertas.sql#evaluar_alertas"}, nil); err != nil {
		t.Fatalf("anclar a un símbolo declarado tiene que funcionar, y falló: %v", err)
	}

	var fp string
	if err := engine.db.QueryRow(
		`SELECT fingerprint FROM observation_origins WHERE observation_id='O1'`).Scan(&fp); err != nil {
		t.Fatal(err)
	}
	// El prefijo NO es cosmético: es lo que hace que el re-cálculo elija la misma fuente. Ver
	// TestElAnclaDeclaradaNoSeVuelveRanciaCuandoElExtractorEmpiezaAEntender.
	if !strings.HasPrefix(fp, prefijoDeclarado) {
		t.Errorf("el fingerprint tiene que declarar su fuente con el prefijo %q, obtuve %q", prefijoDeclarado, fp)
	}
}

// Y SIGUE SIENDO UN ERROR CUANDO NO HAY DE DÓNDE SACARLO.
//
// El fallback no puede convertirse en «cualquier cosa ancla»: sin gist declarado, o con un
// símbolo que el gist no nombra, el error tiene que seguir estando. Si no, una nota se ancla a un
// símbolo inexistente y nace marcada como rancia, que es lo que el mensaje de error evita.
func TestSinSimboloDeclaradoNiExtraidoElAnclaSigueSiendoError(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"supabase/migrations/alertas.sql": contenidoSQL})
	declararGist(t, engine, "supabase/migrations/alertas.sql", "evaluar_alertas L6-11")

	err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "n", 1.0, "", "local",
		[]string{"supabase/migrations/alertas.sql#NoEstaDeclarado"}, nil)
	if err == nil {
		t.Fatal("anclar a un símbolo que no está ni en el archivo ni en el gist tiene que fallar")
	}
	if !strings.Contains(err.Error(), "NoEstaDeclarado") {
		t.Errorf("el error tiene que nombrar el símbolo, obtuve: %v", err)
	}
}

// EL EXTRACTOR MANDA: un archivo Go NO usa lo declarado, ni aunque el gist mienta.
//
// Es la mitad que garantiza que esto no rompe nada. Si lo declarado pudiera ganarle al extractor,
// un gist viejo o mal escrito cambiaría el fingerprint de anclas de Go que hoy funcionan — una
// marca de rancio masiva sobre el único lenguaje donde todo andaba bien.
func TestEnGoElExtractorLeGanaALoDeclarado(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{
		"src/p.go": "package p\n\nfunc Dos() int {\n\treturn 2\n}\n",
	})
	// Un gist que declara el MISMO símbolo en una línea equivocada. Si se usara, el fingerprint
	// sería otro.
	declararGist(t, engine, "src/p.go", "Dos L1")

	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "n", 1.0, "", "local",
		[]string{"src/p.go#Dos"}, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	var fp string
	if err := engine.db.QueryRow(`SELECT fingerprint FROM observation_origins WHERE observation_id='O1'`).Scan(&fp); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(fp, prefijoDeclarado) {
		t.Error("en Go el fingerprint tiene que salir del extractor, no del gist: lo declarado es fallback, no reemplazo")
	}
	derivado, err := symbolFingerprint(root, "src/p.go", "Dos")
	if err != nil {
		t.Fatal(err)
	}
	if fp != derivado {
		t.Errorf("el fingerprint guardado tiene que ser exactamente el derivado del archivo:\n  guardado: %s\n  derivado: %s", fp, derivado)
	}
}

// EL RIESGO QUE ESTE COMMIT CREA, Y LA PRUEBA QUE LO CIERRA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// UN CAMBIO DE BUILD NO PUEDE MARCAR RANCIA A MEDIA MEMORIA
//
// Las anclas declaradas se capturan porque el extractor no entiende ese lenguaje. Pero el
// extractor CAMBIA con el binario: la fase anterior le puso `-tags treesitter` a construir.sh, y
// a partir de ahí un `.tsx` que antes no daba símbolos ahora sí da. Si el re-cálculo eligiera
// siempre «lo mejor disponible», todas esas anclas cambiarían de fuente a la vez, el hash sería
// otro, y saltarían a rancias de golpe sin que nadie tocara una línea de código.
//
// Una marca de rancio masiva y falsa es peor que no tener marca: entrena a ignorarla.
//
// Acá se simula el escenario exacto —se captura sin extractor, después el archivo pasa a ser uno
// que el extractor SÍ entiende— y se exige que el ancla siga sana.
//
// Sabotaje: hacer que originFingerprint ignore el prefijo del fingerprint guardado y use siempre
// fuenteAuto → esta prueba se pone roja.
func TestElAnclaDeclaradaNoSeVuelveRanciaCuandoElExtractorEmpiezaAEntender(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"supabase/migrations/alertas.sql": contenidoSQL})
	declararGist(t, engine, "supabase/migrations/alertas.sql", "evaluar_alertas L6-11; limpiar_alertas L13-18")

	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "n", 1.0, "", "local",
		[]string{"supabase/migrations/alertas.sql#evaluar_alertas"}, nil); err != nil {
		t.Fatalf("save: %v", err)
	}

	// El archivo NO cambia: el re-chequeo tiene que dar sano, eligiendo la misma fuente con la
	// que se capturó. Esto es lo que se rompería si el re-cálculo ignorara el prefijo.
	if stale, err := engine.staleOriginsFor([]string{"O1"}, root); err != nil {
		t.Fatal(err)
	} else if len(stale["O1"]) != 0 {
		t.Fatalf("el ancla no debe estar rancia: nada cambió en el archivo. Obtuve %+v", stale["O1"])
	}

	// Y AHORA SÍ cambia el código que la nota describe: el ancla tiene que saltar.
	nuevo := strings.Replace(contenidoSQL, "perform 1;", "perform 2;", 1)
	if err := os.WriteFile(filepath.Join(root, "supabase", "migrations", "alertas.sql"), []byte(nuevo), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := engine.staleOriginsFor([]string{"O1"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale["O1"]) != 1 || stale["O1"][0].Reason != StaleChanged {
		t.Errorf("cambió el cuerpo de evaluar_alertas y el ancla tiene que marcarse como cambiada; obtuve %+v", stale["O1"])
	}
}

// Y el ancla declarada tiene que ser INSENSIBLE a lo que pasa fuera de su rango, igual que la
// derivada. Si no, no gana nada sobre anclar al archivo entero y el ruido vuelve por otro lado.
func TestElAnclaDeclaradaNoSaltaPorCambiosFueraDeSuRango(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"supabase/migrations/alertas.sql": contenidoSQL})
	// limpiar_alertas declara su rango explícito 13-18.
	declararGist(t, engine, "supabase/migrations/alertas.sql", "evaluar_alertas L6-11; limpiar_alertas L13-18")

	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "n", 1.0, "", "local",
		[]string{"supabase/migrations/alertas.sql#limpiar_alertas"}, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Cambia el cuerpo de evaluar_alertas, que NO es lo que la nota describe.
	nuevo := strings.Replace(contenidoSQL, "perform 1;", "perform 2;", 1)
	if err := os.WriteFile(filepath.Join(root, "supabase", "migrations", "alertas.sql"), []byte(nuevo), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := engine.staleOriginsFor([]string{"O1"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale["O1"]) != 0 {
		t.Errorf("cambió OTRO símbolo del archivo y esta ancla no debería moverse; obtuve %+v", stale["O1"])
	}
}
