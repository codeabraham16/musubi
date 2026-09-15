# motivo.awk — elige el MOTIVO de cada rojo para sabotaje.sh.
#
# Entrada: dos archivos. El primero es la corrida de CONTROL (verde, con -v); el segundo, la
# corrida con el sabotaje puesto. Salida: una línea «Prueba · archivo_test.go:N: mensaje» por fallo,
# con la primera línea de ese fallo que NO aparece en el control. La resta y su frontera están
# explicadas en sabotaje.sh, junto al lugar donde se llama.
#
# EL MENSAJE VA ENTERO, Y ESO NO ES DESCUIDO DE ANCHO. Antes se imprimía `substr(linea,1,150)`, y
# `arnes` usa esta línea como CLAVE para contar «motivos repetidos». Un mensaje de este repo pone el
# dato que distingue AL FINAL —casi siempre interpolado—, así que dos rojos distintos con los mismos
# primeros 150 caracteres se contaban como uno. Medido el 2026-09-14: tres sabotajes de
# ciclos_de_fondo_test.go caían con «…adentro de un *ast.GoStmt», «*ast.DeferStmt» e
# «*ast.IfStmt», y el corte dejaba afuera justo esa palabra. Mismo paquete, mismas 19 corridas: con
# el recorte «motivos repetidos: 1», sin él «0». Y no era un caso raro: 47 de los 126 motivos del
# árbol tocaban el tope. Un recorte usado como clave no avisa: convierte «no lo miré» en «son iguales».
#
# Vive en su propio archivo para que la prueba que lo custodia corra ESTE programa y no una copia.
NR==FNR{ if (/_test\.go:[0-9]+: /){ sub(/^[ \t]+/,""); sub(/^[^ ]*_test\.go:[0-9]+: /,""); base[$0]=1 } ; next }
     /^\s*--- FAIL: /{t=$3; got=0; next}
     t!="" && got==0 && /_test\.go:[0-9]+: /{
         sub(/^[ \t]+/,""); linea=$0; msj=$0; sub(/^[^ ]*_test\.go:[0-9]+: /,"",msj)
         if (msj in base) next
         print "      " t " · " linea; got=1 }
