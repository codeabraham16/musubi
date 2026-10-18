package publico

import (
	"context"
	"fmt"
	"os"

	"musubi/internal/memory"
	"musubi/internal/recalleval"
)

// RankingsMusubi siembra el corpus de UNA pregunta en una base de Musubi NUEVA y devuelve, por cada
// config, el ranking de la consulta en posiciones del corpus. También devuelve cuántos docs no
// entraron a la base (ver abajo).
//
// LA BASE LA CREA Y LA BORRA ESTA FUNCIÓN, Y NO RECIBE DIRECTORIO. Un adaptador que siembra miles
// de sesiones ajenas no puede tener un camino que termine en `.musubi/memory.db` de un proyecto: si
// el directorio viniera de afuera, «nunca en la memoria real» sería una convención del que llama.
// Acá es la única forma posible: un directorio temporal recién creado, la plantilla ya migrada de
// las pruebas (sin migrar de cero en cada pregunta) y RemoveAll al salir.
//
// Siembra con recalleval.SeedEngine y rankea con recalleval.RankingDe: el mismo camino que mide el
// banco, no una copia. Con embed != nil los docs llevan vector y las configs híbridas lo usan; las
// léxicas corren sobre la misma base sin tocarlo.
//
// VIVE EN UN _test.go A PROPÓSITO. La plantilla es de PRUEBAS: es segura porque producción no la
// conoce, y TestLaPlantillaDePruebasNoTieneLlamadorDeProduccion (internal/memory) rechaza cualquier
// llamador fuera de un archivo de prueba. Este adaptador nació en un .go común y esa guarda lo
// habría frenado en CI. Lo usan sólo las pruebas de este paquete, así que su lugar es el de ellas.
func RankingsMusubi(ctx context.Context, consulta string, c Corpus, cfgs []recalleval.Config, embed recalleval.EmbedFunc) (rankings map[string][]int, omitidos int, err error) {
	dir, err := os.MkdirTemp("", "musubi-vara-publica-*")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(dir)
	if err := memory.SembrarPlantillaDePruebas(dir); err != nil {
		return nil, 0, fmt.Errorf("plantilla de la base temporal: %w", err)
	}

	fx := &recalleval.Fixture{Docs: make([]recalleval.Doc, len(c.IDs))}
	for i := range c.IDs {
		fx.Docs[i] = recalleval.Doc{ID: c.IDs[i], Topic: TopicoLongMemEval, Content: c.Textos[i]}
	}
	eng, err := recalleval.SeedEngine(dir, fx, embed)
	if err != nil {
		return nil, 0, err
	}
	defer eng.Close()

	// LOS OMITIDOS SE CUENTAN EN LA BASE, NO SE PREDICEN. SeedEngine saltea el contenido que el motor
	// rechaza por diseño (memory.ErrPayloadInvalido) y sólo lo anota en el log. Volver a preguntar
	// acá con la misma guarda sería una copia de la regla que envejece aparte; contar lo que quedó
	// guardado es el hecho. Más de lo sembrado querría decir que la base no nació vacía.
	n, err := eng.CountSavedItems()
	if err != nil {
		return nil, 0, err
	}
	if n > len(c.IDs) {
		return nil, 0, fmt.Errorf("la base temporal tiene %d items y se sembraron %d: no nació vacía", n, len(c.IDs))
	}
	omitidos = len(c.IDs) - n

	rankings = make(map[string][]int, len(cfgs))
	for _, cfg := range cfgs {
		ids, err := recalleval.RankingDe(ctx, eng, consulta, cfg, embed, len(c.IDs))
		if err != nil {
			return nil, 0, fmt.Errorf("config %s: %w", cfg.Name, err)
		}
		pos, err := c.Posiciones(ids)
		if err != nil {
			return nil, 0, fmt.Errorf("config %s: %w", cfg.Name, err)
		}
		rankings[cfg.Name] = pos
	}
	return rankings, omitidos, nil
}
