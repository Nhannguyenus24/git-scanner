# Uso de GitScanner

GitScanner clona el historial completo de un repositorio, examina cada blob único y
reporta las credenciales que encuentra, tanto en el árbol actual como en cualquier punto del pasado.
Esta página es la guía rápida para usar el panel de control.

## Conexión

La dirección de la API se encuentra en la barra superior; el indicador circular junto a ella
está en verde cuando el panel puede comunicarse con ella. Cámbiala a la dirección de tu propio host
de API si no estás utilizando el valor predeterminado `http://localhost:8080`. La dirección,
el tema y el idioma se recuerdan en este navegador.

Usa los controles en la esquina superior derecha para alternar entre el **modo claro y oscuro** y
para seleccionar tu **idioma**.

## Iniciar un escaneo

Haz clic en **Nuevo escaneo** (arriba a la izquierda o en la barra lateral). Elige qué escanear:

- **Repositorio** — una URL de clonación HTTPS, p. ej. `https://github.com/owner/repo`.
- **Usuario** u **Organización** — todos los repositorios pertenecientes a esa cuenta. Marca
  *incluir repositorios bifurcados (forks)* para escanear también los repositorios derivados.

El escaneo se ejecuta en segundo plano. La página muestra la vista detallada del escaneo, donde
la barra de progreso y los contadores se actualizan a medida que se procesan los repositorios.

## Interpretación de los resultados

El **Resumen** muestra los datos principales: hallazgos abiertos, cuántos son críticos, cuántos
siguen activos en HEAD y los desgloses por gravedad, proveedor y estado de clasificación.
Cada tarjeta y barra es un enlace directo a la lista filtrada de Hallazgos.

La sección de **Analítica** profundiza más: una cronología del momento en que se introdujeron
las credenciales y desgloses por tipo de aplicación y regla de detección.

**Hallazgos** es la lista completa y filtrable. Filtra por gravedad, proveedor, estado o texto libre;
el filtro reside en la URL, por lo que una vista filtrada se puede compartir o guardar en marcadores.
Cada fila muestra:

- la gravedad y el proveedor al que pertenece la credencial;
- una vista previa enmascarada (el secreto real nunca sale del motor);
- el archivo y la línea, con un enlace al archivo en GitHub;
- el commit que la introdujo, su autor y la fecha;
- **Exposición** — `en HEAD` significa que aún está en el árbol actual; `historial`
  significa que se eliminó pero sigue siendo accesible al clonar;
- una insignia **Activo — rotar** cuando la validación en vivo confirmó que la clave aún funciona.

Usa el menú al final de cada fila para marcar un hallazgo como **Rotado**, **Falso positivo** o
**Ignorado** a medida que los revises.

## Repositorios y patrones sin asignar

**Repositorios** clasifica cada repositorio escaneado según su exposición abierta, desglosada
por gravedad. **Sin asignar** es la lista pendiente de secretos de alta entropía que ninguna regla
conocida ha reclamado aún, agrupados por patrón; un patrón frecuente representa un proveedor para
el cual vale la pena redactar una regla.

## Validación activa

Cuando el administrador la ha activado para sus propios objetivos, GitScanner realiza una llamada
de identidad de solo lectura por hallazgo para confirmar si la clave todavía funciona y a qué cuenta
pertenece. Una clave confirmada como activa es la primera que se debe rotar. Está desactivada de forma
predeterminada y nunca enumera el contenido de la cuenta.
