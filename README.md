# MySYS — Sistema de Facturación (versión Go)

Migración completa del sistema MySYS de Python/FastAPI a Go. **Un solo
ejecutable** reemplaza a Python + uvicorn + py2exe: sin dependencias, sin
instalación, y con las **mismas bases de datos SQLite** (tus datos actuales
siguen funcionando sin cambios). Las plantillas usan **`html/template`
nativo de Go** (convertidas de Jinja2, originales en `templates_jinja_original/`).

## Qué incluye (paridad 1:1 con la versión Python)

| Módulo | Estado |
|---|---|
| Login / logout / sesiones firmadas (puerto 8000) | ✅ |
| App de registro del primer admin (puerto 8001) | ✅ |
| Panel admin: dashboard, ventas, productos, clientes, auditoría, usuarios, estadísticas, en línea, archivos, chats, reportes | ✅ |
| Punto de venta `/user/vender` y sala de empleados | ✅ |
| WebSockets: presencia, en línea, chats, archivos | ✅ |
| Panel "Miembros del equipo" en chats: todos los usuarios, fotos, filtro y puntos "en línea" | ✅ |
| **Chats privados 1 a 1** (clic en un usuario → sala `dm:a:b` que solo esos 2 pueden leer) | ✅ NUEVO |
| Escáner de códigos de barras (login, verify, logout, transmisión, lectura, status) | ✅ |
| Reportes JSON + exportación a **Excel (.xlsx)** y **PDF** | ✅ |
| Auditoría con los mismos textos y formato de fecha | ✅ |
| Redondeo de montos idéntico a Python (`Decimal`, ROUND_HALF_UP) | ✅ |
| 24 plantillas convertidas a `html/template` | ✅ |

## Estructura

```
mysys-go/
├── main.go               # Arranque: servidor 8000 + registro 8001 (era main.py)
├── server.go             # Router principal y middleware (era server.py)
├── handlers.go           # Vistas y formularios
├── handlers_api.go       # API REST (ventas, clientes, escáner)
├── handlers_reportes.go  # Estadísticas y reportes
├── ws.go                 # WebSockets (presencia, chats, archivos)
├── export_excel.go       # Exportación .xlsx (era pandas/openpyxl)
├── export_pdf.go         # Exportación .pdf (era reportlab)
├── register.go           # App de registro (era register.py)
├── utils.go              # Sesiones, plantillas (html/template), IP local, helpers
├── helpers.go            # Tokens y utilidades de archivos
├── database/             # Capa de datos SQLite (mismo esquema que Python)
│   ├── db.go             # Conexión y rutas (database/datos_facturacion/…)
│   ├── login.go          # Usuarios          ┐
│   ├── empleados.go      # Empleados         │
│   ├── productos.go      # Inventario        │
│   ├── clientes.go       # Clientes          │ 1:1 con database/*.py
│   ├── ventas.go         # Ventas (versión 19K) │
│   ├── auditoria.go      # Auditoría         │
│   ├── chats.go          # Mensajería        │
│   └── archivos.go       # Archivos          ┘
├── templates/            # 24 plantillas .html en sintaxis GO (listas)
├── templates_jinja_original/  # Respaldo de las plantillas Jinja2 originales
├── tools/                # transpilar.py (Jinja2→Go) y post_ediciones.py
├── static/               # ← COPIA AQUÍ tu carpeta static/ (css/js/img)
└── templates_check_test.go  # go test: compila y ejecuta las 24 plantillas
```

## Compilar

Requiere Go 1.22+ (https://go.dev/dl). No necesita CGO ni gcc.

```bash
cd mysys-go

# Ejecutable para tu sistema actual
go build -o mysys .

# Ejecutable para Windows (desde cualquier SO, reemplaza a py2exe)
GOOS=windows GOARCH=amd64 go build -o mysys.exe .

# Verificación de plantillas (parsea y ejecuta las 24 con datos de muestra)
go test -run TestTemplatesCompile -v
```

## Ejecutar

1. Copia tu carpeta `static/` (de la versión Python) junto al ejecutable.
   Si tienes datos, copia también tu carpeta `database/`.
2. Ejecuta:

```bash
./mysys        # Linux/Mac
mysys.exe      # Windows
```

- Sistema principal: http://localhost:8000 (y `http://<tu-ip>:8000` en la red local)
- Registro inicial: http://localhost:8001 (crea el primer usuario **admin**)
- Ctrl+C detiene ambos servidores limpiamente.
- Si editas un `.html` solo **recarga la página** (el servidor re-parsea al
  detectar cambios en disco).

## Plantillas: Jinja2 → html/template

Las 24 plantillas fueron convertidas automáticamente (`tools/transpilar.py`)
y verificadas renderizando. Equivalencias aplicadas:

| Jinja2 (antes) | Go html/template (ahora) |
|---|---|
| `{{ username }}` | `{{ .username }}` |
| `{% if x %}…{% elif y %}…{% else %}…{% endif %}` | `{{ if .x }}…{{ else if .y }}…{{ else }}…{{ end }}` |
| `{% for v in lista %}…{% else %}…{% endfor %}` | `{{ range $v := .lista }}…{{ else }}…{{ end }}` |
| `{{ loop.index }}` | `{{ inc $i }}` (range con índice) |
| `{% set x = v %}` | `{{ $x := v }}` |
| `{% include "x.html" %}` | `{{ template "x.html" . }}` |
| `{# comentario #}` | `{{/* comentario */}}` |
| `{{ A if C else B }}` | `{{ if C }}{{ A }}{{ else }}{{ B }}{{ end }}` |
| `{{ A or 'texto' }}` | `{{ or .A "texto" }}` (misma semántica) |
| `{{ a == 'x' }}`, `<`, `>=`… | `eqx`, `ltx`, `gex`… (coercionan tipos) |
| `{{ x|upper }}` / `|length` / `|round(2)` | `upper .x` / `len .x` / `round .x 2` |
| `{{ "%.2f"|format(v) }}` | `{{ printf "%.2f" .v }}` |
| `{{ '{:,.2f}'.format(v) }}` | `{{ miles .v }}` |
| `{{ v|tojson }}` | `{{ tojson .v }}` (seguro en `<script>`) |
| `{{ u[0] }}` / `{{ s[:2] }}` | `{{ at $u 0 }}` / `{{ slice $s 0 2 }}` |
| `{{ url_for('static', path='x') }}` | ruta literal `/static/x` |
| `{{ request.query_params.get('error') }}` | `{{ .error }}` (el handler lo pasa) |
| `{{ venta.fecha.strftime('%d/%m/%Y') }}` | `{{ $venta.fecha_completa_dt.Format "02/01/2006 15:04" }}` |

Notas:
- Las variables de la página se acceden con `.` (`.username`) y las de los
  bucles con `$` (`$venta.total`); la raíz dentro de un bucle es `$.`.
- Los valores nulos se imprimen vacíos (configurado con `missingkey=zero`).
- Los floats se imprimen compactos (`3.44`, `6.9`) como en Python.
- `admin_archivos.html` usa las claves alias `tamano_bytes` /
  `tamano_formateado` (sin ñ; el servidor las duplica).

## Notas de migración

- **Base de datos**: mismo esquema y mismas rutas (`database/datos_facturacion/datos/facturacion.db`,
  `…/usuarios/users.db`, `…/chats/chats.db`, `database/mysys.db`). Puedes
  copiar tu carpeta `database/` existente y todo seguirá igual.
- **Contraseñas**: se mantienen en texto plano, exactamente como la versión
  Python (compatible con tus usuarios actuales). Considera hashearlas en el futuro.
- **Clave de sesión**: está en `utils.go` (`LLAVE_SECRETA`), con el mismo valor
  que tu `herramientas/secret_key.py`. Las sesiones Python existentes no se
  trasladan (formato distinto): los usuarios deberán volver a iniciar sesión.
- **`mi_ip.py` / `py2exe_helper.py` / `secret_key.py`**: integrados en
  `utils.go` (`obtenerIPLocal`, `resourcePath`, `LLAVE_SECRETA`).
- **Duplicados**: de los dos `ventas.py` se migró el más reciente (19 KB);
  los dos `main.py` eran idénticos.
- **Panel de mensajes**: las vistas `/admin/chats` y `/user/chats` incluyen el
  panel **Miembros del equipo** bajo "Equipo general"
  (`templates/_equipo_chat.html`, autocontenido): todos los usuarios con foto
  (o inicial), puesto, rol, punto verde si están en línea (se actualiza cada
  15 s vía `/api/usuarios/en_linea`) y un cuadro para **filtrar** por nombre,
  usuario o puesto.
- **Chats privados (nuevo)**: hacer clic (o Enter) en un miembro abre una
  conversación privada 1 a 1:
  - La sala se llama `dm:<usuarioA>:<usuarioB>` (ordenados alfabéticamente).
  - **El servidor la valida**: nadie más puede unirse, leer el historial ni
    enviar mensajes a ella (`puedeEntrarSala` en `ws.go`). Las demás salas
    (como `general`) siguen siendo públicas para el equipo.
  - El WebSocket `/ws/chats` ahora admite **varias salas por conexión** y cada
    mensaje viaja con su `sala` para que el cliente lo encamine.
  - La lógica de cliente compartida vive en `static/js/chat_core.js`
    (reconexión, badges de no leídos por sala, avisos toast, vista previa del
    último mensaje); `chats.js` y `empleado_chat.js` son ahora envoltorios
    delgados.
  - `GET /api/chats/privados` devuelve tus conversaciones privadas con el
    último mensaje; las salas previas aparecen precargadas en la barra lateral
    al entrar a Chats.
  - Correcciones heredadas: `empleado_chat.html` no tenía `data-usuario` en el
    `<body>` (los mensajes propios no se marcaban como "mine"), `admin_chats.html`
    traía un `html` suelto antes del `<!DOCTYPE>` (forzaba quirks mode) y no
    cargaba `presencia.js`, y `empleado_chat.html` traía una `s` al final.
- **Regenerar plantillas**: si cambias las originales Jinja, vuelve a correr
  `python3 tools/transpilar.py templates && python3 tools/post_ediciones.py`.

## Dependencias (se descargan solas con `go build`)

- `modernc.org/sqlite` — SQLite puro en Go (sin CGO → compilación cruzada fácil)
- `go-chi/chi` — router HTTP
- `gorilla/sessions` + `gorilla/websocket` — sesiones y WebSockets
- `xuri/excelize` — Excel · `jung-kurt/gofpdf` — PDF · `shopspring/decimal` — redondeo exacto
- Plantillas: **`html/template` de la librería estándar** (sin dependencias)
