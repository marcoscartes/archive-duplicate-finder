# Gallery View - Nueva Funcionalidad

## Descripción
Se ha añadido una nueva vista de galería que muestra todos los archivos en una cuadrícula 3x3 con previsualizaciones inteligentes.

## Características Implementadas

### 1. **Vista de Galería**
- Nueva página accesible desde el botón "Gallery View" en el header principal
- Cuadrícula responsive 3x3 que se adapta al tamaño de pantalla
- Diseño moderno con efectos hover y transiciones suaves

### 2. **Previsualizaciones Inteligentes**
El sistema busca previsualizaciones en el siguiente orden:
1. **Primera opción**: Imagen JPG más grande (en bytes) dentro del archivo RAR/ZIP/7Z
2. **Fallback**: Si no hay imágenes, busca archivos STL que contengan las palabras clave:
   - "full"
   - "whole"
   - "body"
   - "complete"

### 3. **Lazy Loading (Carga Perezosa)**
- Solo se cargan las previsualizaciones de los archivos visibles en pantalla
- Utiliza Intersection Observer API para detectar cuando un elemento entra en el viewport
- Pre-carga con 200px de margen antes de que el elemento sea visible
- Optimiza el rendimiento significativamente cuando hay muchos archivos

### 4. **Buscador**
- Campo de búsqueda en tiempo real
- Filtra por nombre de archivo o ruta completa
- Actualización instantánea de resultados

### 5. **Acciones por Archivo**
- **Reveal in Folder**: Abre el explorador de archivos en la ubicación del archivo
- **Delete/Trash**: Mueve el archivo a la papelera con confirmación
- Botones visibles al hacer hover sobre cada miniatura

### 6. **Optimizaciones de Rendimiento**
- Carga solo elementos visibles (lazy loading)
- Liberación automática de URLs de blob cuando se desmontan componentes
- Cuadrícula limitada a 3 columnas para mantener tamaños de thumbnail óptimos
- Scroll infinito natural sin paginación compleja

## Endpoints API Nuevos

### `/api/all-files`
- **Método**: GET
- **Descripción**: Retorna todos los archivos únicos de los grupos de tamaño y similitud
- **Respuesta**:
```json
{
  "files": [
    {
      "name": "archivo.rar",
      "path": "C:\\ruta\\archivo.rar",
      "size": 1024000,
      "mod_time": "2026-01-12T10:00:00Z"
    }
  ],
  "total": 150
}
```

### `/api/preview` (Actualizado)
- **Método**: GET
- **Query Params**: `path` (ruta del archivo)
- **Descripción**: Ahora soporta tanto imágenes como archivos STL
- **Content-Type**: 
  - `image/jpeg`, `image/png`, `image/webp` para imágenes
  - `model/stl` para archivos STL

## Funciones Backend Nuevas

### `FindPreviewInArchive(archivePath string)`
Función principal que implementa la lógica de búsqueda inteligente:
1. Intenta encontrar la imagen más grande
2. Si falla, busca STL con palabras clave

### Funciones auxiliares:
- `isSTLFile(filename string)`: Verifica si un archivo es STL
- `hasKeyword(filename string)`: Verifica si contiene palabras clave
- `findKeywordSTLZIP/RAR/7Z`: Busca STL con keywords en cada tipo de archivo

## Uso

1. **Acceder a la galería**: Click en el botón "Gallery View" en el header principal
2. **Buscar archivos**: Usar el campo de búsqueda en la parte superior
3. **Ver preview**: Hacer hover sobre cualquier miniatura
4. **Acciones**: Hacer hover y usar los botones de la esquina superior derecha
5. **Volver**: Click en el botón de flecha izquierda o navegar a "/"

## Notas Técnicas

- La cuadrícula es responsive: 1 columna en móvil, 2 en tablet, 3 en desktop
- Cada thumbnail tiene aspect-ratio 1:1 (cuadrado)
- Los previews se cargan de forma asíncrona y no bloquean la UI
- El componente usa React hooks modernos (useRef, useCallback, useEffect)
- Implementa cleanup apropiado de recursos (URL.revokeObjectURL)

## Mejoras Futuras Posibles

- [ ] Filtros adicionales (por tipo de archivo, tamaño, fecha)
- [ ] Ordenamiento (por nombre, tamaño, fecha)
- [ ] Vista de detalles expandida al hacer click
- [ ] Selección múltiple para acciones en lote
- [ ] Thumbnails en caché para mejorar velocidad de carga repetida
- [ ] Soporte para más formatos de preview (PDF, videos, etc.)
