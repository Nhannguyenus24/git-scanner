# Uso de GitScanner

GitScanner clona el historial completo de un repositorio, lee una sola vez cada
versión única de archivo (blob) e informa de las credenciales que encuentra —
en el árbol actual y en cualquier punto del pasado. Una clave borrada hace tres
años sigue siendo legible para cualquiera que clone el repositorio, así que
sigue siendo una filtración hasta que se revoque.

Esta guía recorre el panel página por página. En resumen:

1. Comprueba que el punto junto a la dirección de la API en la barra superior esté en verde.
2. Pulsa **Nuevo escaneo** e indica un repositorio, un usuario o una organización.
3. Sigue la página del escaneo hasta que termine.
4. Revisa los **Hallazgos**: rota lo que esté activo y marca lo que no sea una filtración.
5. Avisa a los propietarios con un correo de aviso desde **Repositorios**.

---

## La pantalla

### Barra superior

- **Dirección de la API** — el servidor del que procede todo lo que ves. El
  punto junto a ella está en gris mientras se comprueba, en verde cuando la API
  responde y en rojo cuando no se puede alcanzar. Escribe tu propio host si no
  usas el valor predeterminado `http://localhost:8080`; se recuerda en este
  navegador.
- **Comandos** — abre la paleta de comandos (ver abajo).
- **Ayuda** — esta página.
- **Claro / oscuro** — alterna entre papel claro y papel oscuro.
- **Idioma** — English, Tiếng Việt, Italiano, 中文, Español, Deutsch y
  Français. Todo, incluida esta guía, cambia con él.

### Barra lateral

Cada página está a un clic. Tres entradas muestran un recuento en vivo:
**Hallazgos** (hallazgos abiertos), **Escaneos** (escaneos aún en curso) y
**Sin asignar** (patrones a la espera de una regla de proveedor). Pliega la
barra lateral para dejar solo los iconos con la flecha de su parte inferior o
con `Ctrl+[` (`⌘[` en un Mac). En un teléfono la barra lateral se convierte en
una barra inferior; las páginas menos usadas están en **Más**.

### Teclado

- `Ctrl+K` / `⌘K` — abre o cierra la paleta de comandos, desde cualquier sitio.
- `/` — abre la paleta de comandos.
- `N` — inicia un escaneo nuevo.
- `Ctrl+[` / `⌘[` — pliega o despliega la barra lateral.
- `Esc` — cierra un diálogo, un panel lateral o la paleta.

`N` y `/` se ignoran mientras escribes en un cuadro de texto, así que nunca
se comen una pulsación destinada a un filtro.

### Paleta de comandos

Un único cuadro para todo lo que el panel puede hacer desde el teclado. Escribe
para acotar la lista, muévete con `↑` `↓` (o `Ctrl+N` / `Ctrl+P`) y ejecuta con
`Enter`. Ofrece:

- **Acciones** — iniciar un escaneo nuevo; exportar la vista actual.
- **Ir a** — todas las páginas.
- **Filtros** — los filtros predefinidos de hallazgos y los que has guardado,
  aplicados en un solo paso.
- **Apariencia** — claro u oscuro, seguir al sistema, cada tema, densidad,
  movimiento, barra lateral y restablecer.
- **Idioma** — cambia el idioma de la interfaz.

Cualquier otra cosa que escribas se convierte en **Buscar hallazgos con “…”**,
que abre Hallazgos con ese texto como filtro de búsqueda. La paleta no guarda
historial a propósito: un cuadro de búsqueda que recuerda es un lugar donde
podría quedarse una credencial pegada.

Cuando el servidor tiene configurada una clave de la API de Claude, una consulta
de más de una palabra ofrece también **Preguntar a Claude: “…”**. Convierte una
frase como *claves de AWS críticas aún en HEAD en la organización acme* en los
filtros de Hallazgos, abre Hallazgos con ellos y explica cómo interpretó la
pregunta para que puedas corregirla. A Claude solo se envían la pregunta y la
lista de valores de filtro — nunca un hallazgo — y una pregunta que parece
contener una credencial se rechaza antes de enviar nada.

### Introducción

La primera vez que se abre el panel ofrece una introducción de siete pasos.
Omítela cuando quieras y vuelve a verla más tarde desde
**Ajustes → Introducción**.

---

## Iniciar un escaneo

Pulsa **Nuevo escaneo** (barra lateral, página Resumen o Escaneos, o `N`).
Elige un objetivo:

- **Repositorio** — una URL de clonación HTTPS, por ejemplo
  `https://github.com/owner/repo`. Se rechazan las URL SSH y las que contienen
  un nombre de usuario o un token.
- **Usuario** — todos los repositorios de un usuario de GitHub. Un nombre de
  usuario tiene letras, dígitos y guiones, de 1 a 39 caracteres.
- **Organización** — todos los repositorios de una organización de GitHub.
- **Aleatorio** — una muestra aleatoria de repositorios públicos, solo para
  probar la detección. Indica cuántos, o marca **Ejecutar indefinidamente** para
  seguir muestreando hasta que lo pauses o canceles.

Para Usuario, Organización y Aleatorio, marca **Incluir repositorios
bifurcados (forks)** para escanear también los forks; se omiten por defecto
porque un fork repite en su mayor parte a su repositorio de origen.

Pulsa **Iniciar escaneo** o `Enter`. El escaneo se pone en cola en segundo
plano y el panel abre su página de detalle.

---

## Seguir un escaneo

### Lista de escaneos

**Escaneos** muestra cada trabajo aceptado por la API, el más reciente primero.
Cambia entre **Todos**, **Activos** y **Finalizados**. Cada fila muestra el
estado, el objetivo, el progreso, los blobs leídos, los hallazgos nuevos,
cuándo empezó y cuánto duró. Haz clic en una fila para abrirla; los iconos del
final reescanean o eliminan un trabajo terminado.

Un trabajo está en uno de seis estados: **en cola**, **en ejecución**,
**en pausa**, **completado**, **fallido** o **cancelado**.

### Detalle del escaneo

La cabecera indica el objetivo, el tipo de escaneo, cuándo se envió y cuánto
tiempo lleva en marcha. Los botones cambian según el estado:

- **Pausar** (en ejecución) — los workers dejan de tomar repositorios nuevos;
  no se pierde nada de lo ya encontrado. **Reanudar** continúa donde se quedó.
- **Cancelar** (cualquier trabajo sin terminar) — detiene el trabajo
  definitivamente.
- **Reescanear** (terminado) — vuelve a poner en cola el mismo objetivo como un
  trabajo nuevo.
- **Eliminar** (terminado) — borra el trabajo **y todos los hallazgos que
  registró**. No se puede deshacer.

Debajo de la cabecera:

- **Contadores** — repositorios terminados sobre el total, blobs escaneados y
  hallazgos nuevos (vistos por primera vez por este trabajo).
- **Progreso y tiempo restante** — aparece una estimación cuando hay datos
  suficientes; hasta entonces, los contadores son la verdad.
- **Fases** — Descubriendo repositorios → Escaneando repositorios (clonar,
  recorrer el historial, detectar) → Clasificando proveedores → Finalizando
  analítica.
- **Canalización** — un punto por repositorio, situado en el paso que su
  worker informó por última vez (clonar, leer historial, atribuir, verificar,
  guardar) y que avanza cuando se informa el siguiente paso; los repositorios
  terminados caen en el contenedor de la derecha. Naranja significa
  coincidencias candidatas hasta ahora, rojo un intento fallido, gris un
  repositorio omitido.
- **Actividad en vivo** — transmitida desde los workers mientras el escaneo
  está en curso.
- **Resumen final** — cuando el trabajo termina: cuántos hallazgos en cuántos
  repositorios, cuántos son altos o críticos, cuántos siguen siendo alcanzables
  desde la rama por defecto y qué repositorios están afectados.
- **Hallazgos registrados por este escaneo** — la lista en sí, con un enlace
  para abrirla en Hallazgos.

### Actividad en vivo

El panel en vivo muestra lo que hace cada worker en este momento. Su estado
indica **conectando**, **en vivo**, **reconectando**, **terminado** o
**no disponible**; si el servidor no transmite, los contadores de arriba se
siguen actualizando cada pocos segundos.

- **Descubrimiento** — en los escaneos de Usuario y Organización, el listado de
  los repositorios de la cuenta, página a página. *Esperando el límite de
  GitHub* es una pausa, no un fallo.
- **En curso** — cada repositorio y el paso en que se encuentra: **clonar**
  (bare: solo ramas y etiquetas), **leer historial** (cada versión única de
  archivo, con el recuento acumulado de coincidencias candidatas), **atribuir**
  (rastrear cada coincidencia hasta el commit que la introdujo), **verificar**
  (validación en vivo, si está habilitada) y **guardar**.
- **Recién terminados** — repositorios completados hace un momento, con sus
  hallazgos nuevos, los blobs leídos y el tiempo empleado.
- **Registro de actividad** — el flujo completo de eventos; **Mostrar … más**
  lo amplía.

Un repositorio cuyo intento falla se reintenta automáticamente y muestra
*intento 2*, *intento 3*… Un repositorio que falla su último intento se
**omite**; el resto del trabajo continúa.

### Por qué un escaneo puede no encontrar nada nuevo

Una credencial se registra una sola vez, por el primer escaneo que la encontró.
Escanear de nuevo el mismo repositorio encuentra las mismas claves pero no
registra hallazgos *nuevos* — siguen atribuidos al escaneo anterior. Abre
**Hallazgos** filtrados por ese repositorio para verlos todos.

---

## Resumen

La página principal, actualizada cada diez segundos (el indicador **En vivo**
muestra el intervalo).

- **Postura de seguridad** — una puntuación sobre 100. Pulsa **Cómo se calcula**
  para ver cada penalización (abajo).
- **Cifras principales** — hallazgos abiertos, críticos, activos en HEAD,
  repositorios expuestos, escaneos activos, patrones sin asignar.
- **Abiertos por gravedad**, **Abiertos por proveedor**, **Clasificación** —
  desgloses. Cada mosaico y cada barra es un enlace a una lista filtrada de
  Hallazgos.
- **Actividad reciente** — escaneos que terminan o fallan, hallazgos nuevos,
  avisos enviados.
- **Escaneos recientes** — los últimos trabajos y su progreso.

### Diseños

**Diseño** cambia entre cuatro disposiciones: **Todo**, **Triaje diario**
(recuentos y desgloses, sin puntuación), **Resumen ejecutivo** (puntuación,
cifras clave, qué cambió) y **Operaciones de seguridad** (primero actividad y
escaneos en curso). **Personalizar** te permite subir o bajar un panel con las
flechas, ocultarlo con el ojo y recuperar uno oculto desde la lista de abajo.
Los diseños se guardan en este navegador.

### Cómo se calcula la puntuación de postura

La puntuación parte de 100 y pierde puntos por cada factor. Cada penalización
se satura, así que los primeros hallazgos son los que más cuestan y ningún
factor por sí solo puede dispararse:

- **Hallazgos críticos** — hasta 30 puntos.
- **Alcanzables desde HEAD** — hasta 25 puntos. La señal más clara: cualquiera
  que clone hoy la rama por defecto obtiene la credencial.
- **Hallazgos altos** — hasta 15 puntos.
- **Repositorios expuestos** — hasta 15 puntos, según la proporción de
  repositorios escaneados con un hallazgo abierto.
- **Proporción sin resolver** — hasta 10 puntos, según la proporción de
  hallazgos aún abiertos. Un hallazgo cerrado no cuesta nada.
- **Propietarios no avisados** — hasta 5 puntos, por repositorios expuestos sin
  ningún aviso enviado.

De 85 en adelante se lee **Estable**, 65–84 **Mejorable**, 40–64 **Expuesta**,
por debajo de 40 **Crítica**. Si no se ha escaneado nada no hay puntuación, en
lugar de un 100 engañoso. La puntuación se calcula en el navegador a partir de
los contadores de toda la base de datos del servidor, nunca de una sola página
de hallazgos.

---

## Hallazgos

La lista completa de cada credencial registrada, 50 filas por página.

### Filtrar

- **Buscar** — ruta, regla, proveedor o repositorio.
- **Gravedad**, **Proveedor**, **Estado** (Abierto, Rotado, Falso positivo,
  Ignorado).
- **Exposición** — en HEAD, solo en el historial o confirmada activa.
- **Orden** — por defecto, detectados más recientes, commit más antiguo o
  gravedad.
- **Un repositorio** / escaneo — aparece como chip cuando llegas desde un
  repositorio o un escaneo; quítalo para verlo todo.

El filtro vive en la URL, así que una vista filtrada se puede guardar en
marcadores, compartir o reabrir tras recargar. **Borrar filtros** lo
restablece.

Exposición y Orden se calculan en el navegador. Con cualquiera de los dos
activo, la página carga hasta 500 hallazgos del servidor y los refina; si había
más, lo indica encima de la tabla en lugar de mostrar un total engañoso.

### Filtros predefinidos y guardados

El menú **Filtros** ofrece vistas listas para usar:

- **Críticos, aún abiertos** — rota estos primero.
- **Alcanzables desde HEAD** — legibles por cualquiera que clone hoy.
- **Confirmadas activas** — la sonda obtuvo una respuesta válida.
- **Sin atribuir** — ninguna regla de proveedor los reclamó.
- **Descubiertos recientemente** — las detecciones más nuevas primero.
- **Sin resolver más tiempo** — el commit más antiguo primero.

**Guardar el filtro actual** guarda con un nombre el filtro que has
configurado; se puede renombrar o eliminar desde el mismo menú y también
aparece en la paleta de comandos. Los filtros guardados viven solo en este
navegador y contienen solo campos de filtro — nunca un texto de búsqueda que
parezca un secreto pegado.

### Leer una fila

- **Gravedad** y el **proveedor** al que pertenece la credencial (o
  *sin asignar*).
- **Credencial** — una vista previa enmascarada. El valor real nunca sale del
  motor salvo que hayas activado explícitamente el desenmascarado (ver Ajustes).
- **Repositorio** y **Ubicación** — el archivo y la línea, con un enlace al
  archivo en GitHub.
- **Exposición** — `en HEAD` significa que sigue en el árbol actual;
  `historial` significa que se eliminó pero sigue siendo accesible clonando.
  *Commit desconocido: reescanear para atribuir* significa que no se rastreó el
  commit que la introdujo; un reescaneo lo completa.
- **ACTIVO — rotar** — la validación en vivo confirmó que la clave sigue
  funcionando.
- **Estado** y hora de **Detectado**.

El menú al final de una fila marca un hallazgo como **Abierto**, **Rotado**,
**Falso positivo** o **Ignorado**.

### Detalles del hallazgo

Haz clic en una fila para abrir el panel de detalle. Su dirección pasa a la
URL, así que **Copiar enlace** le da a un compañero la misma vista;
**Copiar referencia** copia una referencia corta para un ticket. El panel
muestra:

- **Credencial** — vista previa enmascarada, categoría y vigencia. Cuando el
  valor se guardó para un propietario sin enmascarar y el enmascarado está
  desactivado, **Mostrar** lo revela, marcado como **SIN MÁSCARA**.
- **Cuota y límites** — lo que el proveedor informó durante la verificación de
  vigencia: peticiones o tokens restantes, ámbitos, país, moneda. Una
  instantánea del momento del escaneo.
- **Por qué se detectó** — la regla, el proveedor al que se atribuyó, la
  longitud y el juego de caracteres, una variable o nombre de host cercanos y
  la exposición. La explicación usa solo forma, longitud y contexto, nunca el
  valor.
- **Dónde** — archivo, commit, autor y fecha del commit, con un enlace a la
  línea en GitHub.
- **Cronología** — commit, detección, verificación y decisiones de triaje.
- **Qué hacer** — los pasos de remediación en orden: revocar y volver a emitir
  en el proveedor, revisar su registro de auditoría, eliminar el valor de HEAD,
  decidir si reescribir el historial y añadir una comprobación de secretos
  pre-commit.
- **Mismo repositorio** y **Mismo proveedor** — hallazgos relacionados, a un
  clic.
- **Metadatos** — todo lo demás que registró el motor.

### Triaje en lote

Marca las casillas al inicio de las filas, o la casilla de la cabecera para
seleccionar todas las filas de la página. Aparece una barra en la parte
inferior con el recuento y los cuatro estados. **Rotado** y **Falso positivo**
piden confirmación primero, porque cada uno es una afirmación sobre el mundo
exterior — que la clave se revocó, que la coincidencia era errónea — y en lote
es la afirmación que puede cerrar silenciosamente una filtración real. Abierto
e Ignorado se aplican de inmediato. **Exportar selección** exporta solo las
filas marcadas.

### Exportar

**Exportar** escribe los hallazgos cargados en ese momento, con los filtros que
los produjeron registrados en el archivo:

- **Hallazgos en CSV** — una fila por hallazgo, para una hoja de cálculo.
- **Hallazgos en JSON** — estructurado, para un script.
- **Informe de incidente** — Markdown, agrupado por repositorio.
- **Resumen ejecutivo** — una página, sin tabla.

Las exportaciones solo contienen vistas previas enmascaradas; la credencial en
texto plano nunca se escribe en un archivo, en ningún formato.

---

## Repositorios

Cada repositorio escaneado, ordenado según la gravedad de su exposición
abierta.

- **Filtrar por propietario/nombre** y **Solo con hallazgos abiertos**.
- **Ordenar por** riesgo, hallazgos abiertos, último escaneo o nombre.
- Columnas: tipo, recuentos abiertos por gravedad, abiertos / total,
  **Riesgo**, **Siguiente paso**, **Avisado** y último escaneo.
- Haz clic en un repositorio para abrir sus hallazgos.

El **Riesgo** va de 0 (nada abierto) a 100. Los hallazgos abiertos se ponderan
por gravedad — uno crítico cuenta diez veces uno bajo — y el total se satura,
así que un repositorio con un solo crítico ya aparece arriba. Franjas: Bajo,
Medio (25+), Alto (50+), Crítico (75+). Pasa el cursor para ver los recuentos
que lo componen.

El **Siguiente paso** indica lo único que merece la pena hacer: *Rota las
claves críticas*, *Avisa al propietario*, *Divulgación en cola*, *La
divulgación falló: reintenta*, *Revisa los hallazgos restantes* o *Nada
abierto*.

### Avisar al propietario

El chip **Correo de aviso** de la parte superior indica si el correo está
activado y enlaza a Ajustes. Con el correo activado, **Avisar** en una fila
pone en cola un aviso al propietario del repositorio con la lista de sus
hallazgos en vista previa enmascarada. La columna **Avisado** indica entonces
*enviando*, *enviado*, *fallido* o *excluido*.

### Máquina del tiempo

El icono del reloj en una fila (o el chip **Máquina del tiempo** en Hallazgos
filtrados por un repositorio) reproduce la exposición de ese repositorio commit
a commit. Cada archivo con una credencial tiene su carril; un rombo marca el
commit que introdujo cada clave, y desde ahí una línea llega hasta hoy — continua
mientras la clave sigue en HEAD, desvaneciéndose una vez eliminada.

Arrastra el control deslizante, avanza con los botones anterior y siguiente, o
pulsa reproducir para avanzar desde la primera filtración. En cada paso la
página indica cuántas credenciales se habían confirmado hasta entonces, cuántas
siguen hoy en HEAD y que todas siguen siendo clonables. Debajo se listan los
commits que introdujeron una credencial, del más reciente al más antiguo, con
autor, fecha y un enlace a cada hallazgo.

El escaneo sabe cuándo se confirmó una clave, no cuándo se borró, así que la
línea de una clave eliminada se desvanece en lugar de terminar en una fecha
inventada. Los hallazgos sin fecha de commit se cuentan debajo de la lista; un
nuevo escaneo los sitúa.

---

## Propagación de claves

Credenciales halladas en más de un repositorio. Los hallazgos se registran una
vez por repositorio, así que el mismo secreto en varios sitios solo aparece
aquí, donde las copias se unen por su huella.

- **Claves compartidas**, **Entre propietarios** (claves en repositorios de
  distintos propietarios), **Abiertas en HEAD** y **Repos / propietarios**
  alcanzados. Cuando hay más claves compartidas de las que dibuja el grafo, la
  primera tarjeta indica cuántas se dibujan.
- **Hallada en** — muestra las claves presentes en 2, 3, 5 o 10 repositorios o
  más.
- **El grafo** — un rombo por clave, coloreado por gravedad y con un tamaño
  según cuánto se ha extendido; un círculo por repositorio; un cuadrado por cada
  propietario que tiene dos o más. Una línea roja es una copia aún en HEAD, una
  discontinua una copia ya tratada. Pasa el cursor para iluminar el entorno de
  una clave.
- **La lista** de al lado contiene las mismas claves para el uso con teclado y
  lector de pantalla. Elegir una clave en cualquiera de los dos muestra sus
  repositorios, cada uno enlazado a su hallazgo.

Una sola rotación arregla todas las copias, así que el detalle de una clave
puede **marcar todas las copias** como *Rotado* o *Falso positivo* de una vez,
tras una confirmación. Un valor pegado en repositorios de muchos propietarios
sin relación suele ser código repetitivo que el paso genérico confundió con un
secreto — nombres de clases de un framework, claves de ejemplo de un tutorial —
y esta vista es la forma más rápida de detectarlos.

---

## Sin asignar

La lista pendiente de secretos de alta entropía que ninguna regla de proveedor
con nombre reclama, agrupados por forma. Cada fila muestra un **nombre
sugerido** (tomado de la variable o clave más cercana — a veces solo una
palabra próxima), una **muestra** enmascarada, la **forma** (longitud y juego
de caracteres), el **contexto** en que se encontró, cuántas veces se ha
**visto**, su estado y cuándo se vio por última vez. Una forma que no deja de
aparecer es un proveedor para el que merece la pena escribir una regla. Los
hallazgos de este tipo aparecen como *sin asignar* hasta que una regla los
reclama; cuando ocurre, el siguiente escaneo los vuelve a atribuir.

---

## Proveedores

Cada proveedor que GitScanner puede detectar y atribuir, agrupado por categoría
(IA, nube, datos, pagos, mensajería, DevOps, herramientas para desarrolladores,
otros), con el número de reglas de cada uno. Haz clic en un proveedor para ver:

- **Por qué esta gravedad** — crítica es control total; alta es acceso amplio a
  la API; media es acceso limitado o restringido a un host; baja identifica una
  cuenta pero no puede actuar sobre ella.
- **Qué permite hacer una filtración** — el impacto para esa categoría.
- **Cómo son las claves y cómo se detectan**, una tarjeta por regla:
  - *prefijo propio* — el valor empieza con el prefijo del proveedor, que es
    una prueba por sí mismo;
  - *nombre de ajuste* — un valor sin forma propia, que solo se informa cuando
    se asigna a un ajuste con el nombre del proveedor;
  - *URL de conexión* — una URL con la credencial dentro; una que solo apunta a
    localhost o a un servicio de compose se trata como configuración de
    desarrollo y se descarta;
  - *bloque PEM* — una clave privada de BEGIN a END;
  - *solo estructura* — longitudes fijas, separadores o marcadores, mantenida
    deliberadamente estrecha.
  Cada tarjeta puede mostrar la estructura de la clave, el patrón, el texto que
  debe contener el archivo y una palabra que debe aparecer cerca.
- **Después de una coincidencia** — los marcadores de posición
  (`your-…-key`, `xxxx`, `changeme`), las claves de documentación y de sandbox
  y los datos de prueba se descartan; lo que sobrevive se guarda solo como
  huella y vista previa enmascarada.

Un proveedor muestra su regla más grave. En cada hallazgo, la gravedad sube un
nivel si la filtración sigue siendo alcanzable desde HEAD, y uno más para los
proveedores de nube y de pagos, hasta crítica.

---

## Analítica

Una lectura de todo lo escaneado hasta ahora:

- **Hallazgos** y **Repositorios** — totales, abiertos, expuestos / escaneados.
- **Calendario de fugas** — un cuadrado por día, un año cada vez, sombreado
  según cuántas credenciales se confirmaron ese día y con borde donde una era
  crítica. Elige el año arriba a la derecha. Los días son UTC: la zona horaria
  del autor no se registra, así que un cuadrado puede ser su día anterior o
  siguiente.
- **Cronología de introducción de filtraciones** — hallazgos según el mes en
  que se confirmó la credencial.
- **Abiertos por gravedad**, **Abiertos por proveedor** (proveedores conocidos
  más la categoría sin asignar), **Por tipo de aplicación**, **Estado de
  clasificación**.
- **Reglas de detección** — qué reglas se activan con más frecuencia.
- **Validación de credenciales activas** — activas, revocadas, sin
  comprobador, error de comprobación.
- **Correo de aviso** — repositorios contactados y expuestos aún sin avisar.
- **Repositorios más expuestos** — según la misma puntuación de riesgo que
  Repositorios.
- **Sin resolver más tiempo** — hallazgos abiertos por fecha de commit.

Las cifras son históricas; no hay rango de fechas, porque la API no lo ofrece.
Los filtros de la parte superior guían los enlaces a Hallazgos que abre la
página.

---

## Correos de aviso (Avisos)

Cada aviso enviado a los propietarios de repositorios y su resultado:
repositorio, destinatario, estado (*enviando*, *enviado*, *fallido*,
*excluido*), número de hallazgos, cuándo se solicitó y cuándo se envió. Busca
por repositorio o destinatario.

- **Ver correo** — el mensaje en HTML y en texto plano, regenerado con los
  hallazgos tal como están ahora.
- **Reenviar** — lo vuelve a poner en cola.

---

## Ajustes

### Apariencia

Guardado solo en este navegador.

- **Tema** — Papel (papel envejecido, verde azulado y rojo sangre de buey),
  Medianoche (terminal casi negro, verde fósforo), Océano (azul marino, cian y
  azul), Señal (grafito, solo las alertas llevan color), Minimal (sobrio,
  empresarial).
- **Claro u oscuro** — Claro, Oscuro, o Sistema para seguir a tu sistema
  operativo.
- **Densidad** — Compacta muestra más filas en pantalla.
- **Movimiento** — Reducido desactiva las animaciones.
- **Empezar con la barra lateral plegada**.
- **Restablecer apariencia** — vuelve a los valores predeterminados.

### Introducción

**Repetir la introducción** vuelve a abrir el recorrido de bienvenida.

### Notificaciones del navegador

Con esta opción activada y el permiso del navegador concedido, un panel abierto
— basta una pestaña en segundo plano — te avisa cuando termina un escaneo que
estaba siguiendo y cuando se confirman más claves activas; al hacer clic en la
notificación se abre el escaneo o las claves activas. Mientras estás mirando el
panel, la misma noticia llega como un mensaje dentro de la página. No se envía
nada cuando todas las pestañas del panel están cerradas. Solo se guarda en este
navegador.

### Instalar el panel

El panel se puede instalar como aplicación desde la barra de direcciones del
navegador (o con *Añadir a pantalla de inicio* en un teléfono). Entonces se abre
en su propia ventana y su estructura carga sin red; los datos siguen llegando de
la API y nunca se guardan en la caché del navegador.

### Correo de aviso

El interruptor general. Cuando está activado, Avisar envía un aviso al
propietario del repositorio. Cuando está desactivado no se envía nada y lo que
ya estuviera en cola se descarta. Todavía no existen preferencias por gravedad
ni de resumen periódico.

### Enmascarado de seguridad y propietarios sin enmascarar

**Activado** (predeterminado): todas las credenciales se muestran
enmascaradas. **Desactivado**: las credenciales encontradas en repositorios de
los propietarios listados en **Propietarios sin enmascarar** se muestran
completas, para pruebas; para todos los demás siguen enmascaradas, y el correo
y las exportaciones siempre van enmascarados.

- Añade solo usuarios u organizaciones de GitHub que te pertenezcan.
- Los valores se guardan, cifrados, a partir del siguiente escaneo —
  reescanea los repositorios de un propietario para ver sin máscara los
  hallazgos existentes.
- Quitar un propietario borra todo lo guardado para él.
- Si el servidor no tiene `SECRET_SEAL_KEY`, el desenmascarado no está
  disponible y nunca se guarda nada. Genera una con `openssl rand -hex 32`,
  añádela a `.env.local` y reinicia.

Mientras el enmascarado está desactivado, permanece en la página un aviso
rojo: las credenciales completas de los propietarios listados son visibles para
cualquiera que pueda abrir este panel.

---

## Validación en vivo

Cuando el operador la activa (`PROBE_SECRETS`) para sus propios objetivos,
GitScanner hace una llamada de identidad de solo lectura por hallazgo para ver
si la clave sigue funcionando y de quién es la cuenta:

- **Activo — rotar** — el proveedor la aceptó. Rota esta primero.
- **Revocado** — el proveedor la rechazó; ya rotada o revocada.
- **Sin comprobador** — no existe ninguna comprobación para este proveedor.
- **Error de comprobación** — límite de peticiones o red; prueba a reescanear
  más tarde.

Está desactivada por defecto, nunca enumera el contenido de la cuenta y sus
lecturas de cuota y límites aparecen en el panel del hallazgo.

---

## Gravedad

- **Crítico** — control total: mover dinero, apoderarse de la cuenta, leer todo
  lo que protege.
- **Alto** — acceso amplio a la API como la cuenta.
- **Medio** — acceso limitado o restringido a un host: un canal, un webhook, un
  servidor que además debe ser alcanzable.
- **Bajo** — identifica una cuenta pero no puede actuar sobre ella, como una
  clave en modo de prueba. Aun así conviene rotarla.

---

## Solución de problemas

- **“No se pudo conectar con la API”** — comprueba la dirección de la barra
  superior y que la pila esté en marcha; el punto se pone en verde cuando
  responde.
- **Un escaneo se queda en 0%** — ábrelo y mira la Actividad en vivo.
  *Esperando el límite de GitHub* se resuelve solo; ninguna actividad significa
  que los workers no están en marcha.
- **Un reescaneo no encontró nada nuevo** — los hallazgos pertenecen al
  escaneo anterior; filtra Hallazgos por el repositorio.
- **Commit desconocido** — reescanea el repositorio para atribuirlo.
- **Avisar aparece desactivado** — activa el correo de aviso en Ajustes.
- **Falta Mostrar en el panel** — el enmascarado está activado, el propietario
  no está en la lista sin máscara, el servidor no tiene `SECRET_SEAL_KEY`, o el
  repositorio no se reescaneó después de añadir al propietario.
- **La lista dice que solo refinó 500 hallazgos** — acota los filtros del
  servidor (gravedad, proveedor, estado, repositorio) para que la ventana lo
  cubra todo.
- **No aparece “Preguntar a Claude” en la paleta** — el servidor no tiene
  `ANTHROPIC_API_KEY`, o la consulta es de una sola palabra (eso es una
  búsqueda normal).
- **Preguntar a Claude rechazó la pregunta** — contenía algo con forma de
  credencial; reformúlala sin el valor.
- **No llegan notificaciones del navegador** — revisa el permiso de este sitio
  en el navegador; a un sitio bloqueado no se le puede volver a preguntar desde
  la página.
- **Resultados obsoletos** — eliminar un trabajo o borrar los datos elimina sus
  hallazgos; reescanea para recuperarlos.

GitScanner es solo para pruebas de seguridad autorizadas. Escanea lo que te
pertenece o lo que tienes permiso para probar, e informa a los propietarios de
lo que encuentres.
