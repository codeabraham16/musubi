package codeintel

import "testing"

// LOS CASOS DE ESTA PRUEBA SON GISTS REALES, NO INVENTADOS.
//
// Salen de la base de Altura-erp, escritos por el agente sin que nadie le dictara el formato. Es
// importante que sean los de verdad: un parser probado contra ejemplos prolijos que uno mismo se
// escribe pasa en verde y falla con el primer dato real, y acá el dato real trae nombres con
// espacios, rangos explícitos que se solapan, aproximaciones y basura colgando al final.
func TestParseSymbolLineSobreGistsReales(t *testing.T) {
	casos := []struct {
		nombre  string
		linea   string
		esperar []Symbol
	}{
		{
			nombre: "src/App.jsx tal cual está guardado",
			linea:  "landingForRole L49 (incluye kiosko→/kiosko); RoleProtectedRoute L57; rutas Layout con allowedRoles L80-122 (incluyen ad_ordenes); import Kiosko L34",
			esperar: []Symbol{
				// Ordenado por línea: el import venía último en el texto y arranca antes.
				{Name: "import Kiosko", Kind: KindDeclarado, StartLine: 34, EndLine: 48},
				{Name: "landingForRole", Kind: KindDeclarado, StartLine: 49, EndLine: 56},
				{Name: "RoleProtectedRoute", Kind: KindDeclarado, StartLine: 57, EndLine: 79},
				// Rango EXPLÍCITO: no se pisa con el derivado, y es el último igual.
				{Name: "rutas Layout con allowedRoles", Kind: KindDeclarado, StartLine: 80, EndLine: 122},
			},
		},
		{
			nombre: "el .sql, con un punto que el matcher va a partir igual",
			linea:  "DO block L25; cron.schedule L60 ('0 * * * *')",
			esperar: []Symbol{
				{Name: "DO block", Kind: KindDeclarado, StartLine: 25, EndLine: 59},
				// `cron.schedule` NO es un método, pero se parte igual, y tiene que ser así: del
				// otro lado symbolMatches parte el PEDIDO con el mismo strings.Cut y exige
				// Recv=="cron". Guardarlo entero haría que el ancla no resolviera jamás. La regla
				// no es «adivinar la semántica», es «igualar al que compara».
				{Name: "schedule", Recv: "cron", Kind: KindDeclarado, StartLine: 60, EndLine: 60},
			},
		},
		{
			nombre:  "aproximado: se DESCARTA, no se adivina",
			linea:   "generarPlantillaFisica ~L1375",
			esperar: nil,
		},
		{
			nombre: "calificado Tipo.Metodo, la forma que emite Ref()",
			linea:  "DbEngine.AutoEmbedBackfill L120-140",
			esperar: []Symbol{
				{Name: "AutoEmbedBackfill", Recv: "DbEngine", Kind: KindDeclarado, StartLine: 120, EndLine: 140},
			},
		},
		{
			nombre: "la firma pegada al nombre se saca (formas reales de Altura)",
			linea:  "DetalleBadges() L57; RoleSelector({value,onChange,disabled,title}) L100",
			esperar: []Symbol{
				{Name: "DetalleBadges", Kind: KindDeclarado, StartLine: 57, EndLine: 99},
				{Name: "RoleSelector", Kind: KindDeclarado, StartLine: 100, EndLine: 100},
			},
		},
		{
			nombre:  "sin L: no hay rango que hashear",
			linea:   "algoSinLinea; otro tampoco",
			esperar: nil,
		},
		{
			nombre:  "vacío",
			linea:   "   ",
			esperar: nil,
		},
		{
			nombre: "desordenado y con un solape: el que se solapa NO deriva",
			linea:  "b L50; a L10-100",
			esperar: []Symbol{
				// `a` declara 10-100 explícito y se respeta aunque contenga a `b`.
				{Name: "a", Kind: KindDeclarado, StartLine: 10, EndLine: 100},
				{Name: "b", Kind: KindDeclarado, StartLine: 50, EndLine: 50},
			},
		},
		{
			nombre: "dos en la misma línea: no se deriva un rango invertido",
			linea:  "x L7; y L7",
			esperar: []Symbol{
				{Name: "x", Kind: KindDeclarado, StartLine: 7, EndLine: 7},
				{Name: "y", Kind: KindDeclarado, StartLine: 7, EndLine: 7},
			},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := ParseSymbolLine(c.linea)
			if len(got) != len(c.esperar) {
				t.Fatalf("devolvió %d símbolos y esperaba %d:\n  got: %+v\n  want: %+v", len(got), len(c.esperar), got, c.esperar)
			}
			for i := range got {
				if got[i] != c.esperar[i] {
					t.Errorf("símbolo %d:\n  got:  %+v\n  want: %+v", i, got[i], c.esperar[i])
				}
			}
		})
	}
}

// EL IDA Y VUELTA TIENE QUE CERRAR CONTRA EL EXTRACTOR DE VERDAD.
//
// Es la prueba que impide que las dos mitades se separen: si alguien cambia el formato de
// FormatSymbols, el parser deja de leer lo que la casa escribe y las anclas declaradas se rompen
// en silencio, que es el modo de falla más caro de todos (una marca de rancio que no salta no se
// distingue de una nota que sigue valiendo).
//
// Sabotaje: cambiar el separador de FormatSymbols, o el prefijo `L`.
func TestFormatSymbolsYParseSymbolLineCierranElIdaYVuelta(t *testing.T) {
	origen := []Symbol{
		{Name: "Alpha", Kind: KindFunc, StartLine: 10, EndLine: 20},
		{Name: "Guardar", Recv: "Caja", Kind: KindMethod, StartLine: 30, EndLine: 40},
		{Name: "Beta", Kind: KindType, StartLine: 50, EndLine: 60},
	}
	linea := FormatSymbols(origen)
	vuelta := ParseSymbolLine(linea)

	if len(vuelta) != len(origen) {
		t.Fatalf("FormatSymbols produjo %q y ParseSymbolLine devolvió %d símbolos, esperaba %d", linea, len(vuelta), len(origen))
	}
	for i := range origen {
		if vuelta[i].Name != origen[i].Name || vuelta[i].Recv != origen[i].Recv {
			t.Errorf("no cerró la identidad del símbolo %d: got %s.%s, want %s.%s",
				i, vuelta[i].Recv, vuelta[i].Name, origen[i].Recv, origen[i].Name)
		}
		if vuelta[i].StartLine != origen[i].StartLine {
			t.Errorf("no cerró la línea de inicio del símbolo %d: got %d, want %d", i, vuelta[i].StartLine, origen[i].StartLine)
		}
	}
	// FormatSymbols NO escribe el fin, así que la vuelta no puede recuperarlo: se deriva. Se deja
	// afirmado para que quede claro que es una PÉRDIDA CONOCIDA y no un defecto del parser — y
	// para que si alguien alguna vez le agrega el rango al formato, esta prueba lo obligue a
	// mirar acá.
	if vuelta[0].EndLine != origen[1].StartLine-1 {
		t.Errorf("el fin derivado del primero debería llegar hasta la línea previa al segundo: got %d, want %d", vuelta[0].EndLine, origen[1].StartLine-1)
	}
}
