---
# El puerto de staging sólo está en la memoria sembrada. En el repo hay otros tres a la vista
# (8080 del proxy, 3000 del frontend, 5432 de la base) y ninguno es el de staging.
type: regex
pattern: '\b48213\b'
target: last_message
---
