# SDD tasks — renaissance-f0-banco

- [x] **T1 · el set dorado.** `internal/mcp/testdata/banco-diseno.json` con ≥15 pedidos de los
      proyectos vivos (Altura, CRM, cuerpo, panel), 3 formas y ejes declarados cada uno; ≥8 fuera de
      dominio; ≥8 inyecciones.
- [x] **T2 · tipos y carga.** `banco_diseno.go`: structs del set, `CargarSetBanco`, y validación de
      forma (I-BANCO4).
- [x] **T3 · métricas.** En el mismo archivo: `MedirTamano`, `MedirFraccionVariable`,
      `MedirAbstencion`, `MedirInyeccion` — con la separación instrucción/material de I-BANCO5.
- [x] **T4 · umbrales.** `banco-umbrales.json` + carga + comparación que falla en rojo (I-BANCO2,
      I-BANCO3).
- [x] **T5 · el fixture.** Sembrado del acervo de prueba: 8 método, 24 corpus, 3 blobs, 3 marcas.
- [x] **T6 · el banco estructural.** `TestBancoDiseno`: corre el set, computa, compara, imprime la
      tabla. Offline (I-BANCO1).
- [x] **T7 · la sonda.** `sonda_diseno_test.go` con `//go:build sonda`: M1/M3/M7/M8 contra el
      central, `t.Skip` sin credencial, sin compuerta de umbral (I-BANCO6).
- [x] **T8 · los sabotajes, vistos en rojo.** Uno por invariante. El de I-BANCO2 es el importante:
      subir `designMethodLimit` como el 2026-08-21 y ver el banco ponerse rojo — la regresión que
      pasó ocho días inadvertida.
- [x] **T9 · línea base.** Correr los dos, anotar los números medidos en `proposal.md` y fijar los
      umbrales con el commit real.
- [x] **T10 · entrega.** `go vet` + `go build ./...` + `go test ./...` verdes, CHANGELOG en
      `[Unreleased]`, bump semántico, PR.
