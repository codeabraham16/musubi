# Diseño — Hallazgos congelados

Ver `spec.md`. Las dos decisiones que ordenan el archivo:

1. **`finding.go` no toca `Compute` ni `Check`.** El hook pre-push está vivo. `huellaDe` duplica
   el esquema de longitud-prefijada a propósito, y `TestH0` congela la salida de `Compute` con
   hexes literales por si un refactor futuro la mueve.
2. **El cuerpo no se persiste.** Es lo que obliga a que el verificador tenga el texto.

Alternativas descartadas:

| alternativa | por qué no |
|---|---|
| generalizar la aridad de `Compute` | riesgo de mover un byte y bloquear todos los push |
| guardar el cuerpo | el verificador se miraría al espejo |
| un solo mensaje de rechazo | el agente reintentaría la acción equivocada |
| normalizar espacio/mayúsculas | el congelado aprobaría mutaciones reales |
| el cuerpo por bandera | quedaría en el historial del shell y en los logs |
