package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/sessions"

	"mysys/database"
)

// LLAVE_SECRETA firma las cookies de sesión (la misma de herramientas/secret_key.py).
// Cámbiala por una clave propia en producción.
const LLAVE_SECRETA = "d3dcd565f76c038722d771c8711117b3ef67a05c74c03673ef4d55db7247f698"

// Almacén de sesiones firmadas en cookie (equivalente a SessionMiddleware de Starlette).
var store = sessions.NewCookieStore([]byte(LLAVE_SECRETA))

const nombreCookieSesion = "session"

func init() {
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   0, // cookie de sesión (expira al cerrar el navegador)
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// ── Rutas de recursos (equivalente a py2exe_helper.resource_path) ──────────

// resourcePath resuelve la ruta de un recurso: primero junto al ejecutable
// (modo "empaquetado"), si no, en el directorio de trabajo actual.
func resourcePath(rel string) string {
	if exe, err := os.Executable(); err == nil {
		candidato := filepath.Join(filepath.Dir(exe), rel)
		if _, err := os.Stat(candidato); err == nil {
			return candidato
		}
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, rel)
}

var (
	staticDir    = resourcePath("static")
	templatesDir = resourcePath("templates")

	CarpetaImagenes  = filepath.Join(staticDir, "productos_img")
	CarpetaPerfilImg = filepath.Join(staticDir, "perfil_img")
	CarpetaArchivos  = filepath.Join(staticDir, "archivos_storage")
)

// ── Plantillas (html/template nativo de Go) ────────────────────────────────
// Las plantillas fueron convertidas de Jinja2 a sintaxis Go (ver README).
// Funciones auxiliares disponibles en todas las plantillas: at, eqx, nex,
// gtx, ltx, gex, lex, minN, maxN, upper, lower, title, miles, round, tojson,
// str, toInt, toFloat, safeHTML. Además de las builtins: len, slice, index,
// printf, and, or, not, eq...

var (
	tpl       *template.Template
	tplMu     sync.Mutex
	tplModMax time.Time
)

// aNum coercion a float64 para las funciones de comparación de plantillas.
func aNum(v any) (float64, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
}

func eqx(a, b any) bool {
	if af, ok := aNum(a); ok {
		if bf, ok2 := aNum(b); ok2 {
			return af == bf
		}
	}
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func nex(a, b any) bool { return !eqx(a, b) }

func gtx(a, b any) bool {
	af, _ := aNum(a)
	bf, _ := aNum(b)
	return af > bf
}

func ltx(a, b any) bool {
	af, _ := aNum(a)
	bf, _ := aNum(b)
	return af < bf
}

func gex(a, b any) bool {
	af, _ := aNum(a)
	bf, _ := aNum(b)
	return af >= bf
}

func lex(a, b any) bool {
	af, _ := aNum(a)
	bf, _ := aNum(b)
	return af <= bf
}

func minN(a, b any) float64 {
	af, _ := aNum(a)
	bf, _ := aNum(b)
	return math.Min(af, bf)
}

func maxN(a, b any) float64 {
	af, _ := aNum(a)
	bf, _ := aNum(b)
	return math.Max(af, bf)
}

// at indexa strings (por carácter, como Jinja) y slices/arrays (por elemento).
func at(v any, i int) any {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		r := []rune(t)
		if i >= 0 && i < len(r) {
			return string(r[i])
		}
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		if i >= 0 && i < rv.Len() {
			return rv.Index(i).Interface()
		}
	}
	return ""
}

// miles formatea con separador de miles y 2 decimales ("1,234.56").
func miles(v any) string {
	f, _ := aNum(v)
	signo := ""
	if f < 0 {
		signo = "-"
		f = -f
	}
	entero := int64(f)
	decimales := int64(math.Round((f - float64(entero)) * 100))
	if decimales >= 100 {
		entero++
		decimales = 0
	}
	digitos := strconv.FormatInt(entero, 10)
	var sb strings.Builder
	for i, d := range digitos {
		if i > 0 && (len(digitos)-i)%3 == 0 {
			sb.WriteByte(',')
		}
		sb.WriteRune(d)
	}
	return fmt.Sprintf("%s%s.%02d", signo, sb.String(), decimales)
}

// round redondea half-up como el filtro round de Jinja2.
func round(v any, prec ...int) float64 {
	f, _ := aNum(v)
	p := 0
	if len(prec) > 0 {
		p = prec[0]
	}
	m := math.Pow(10, float64(p))
	return math.Round(f*m) / m
}

// tojson serializa a JSON seguro para incrustar en <script>.
func tojson(v any) template.JS {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		return template.JS("null")
	}
	return template.JS(strings.TrimRight(buf.String(), "\n"))
}

func titulo(s string) string {
	return strings.Join(mapearPalabras(s, strings.ToUpper), " ")
}

func mapearPalabras(s string, f func(string) string) []string {
	palabras := strings.Fields(s)
	for i, p := range palabras {
		r := []rune(p)
		if len(r) > 0 {
			palabras[i] = f(string(r[:1])) + string(r[1:])
		}
	}
	return palabras
}

func safeHTML(s string) template.HTML { return template.HTML(s) }

var funcMap = template.FuncMap{
	"at":       at,
	"eqx":      eqx,
	"nex":      nex,
	"gtx":      gtx,
	"ltx":      ltx,
	"gex":      gex,
	"lex":      lex,
	"minN":     minN,
	"maxN":     maxN,
	"inc":      func(i int) int { return i + 1 },
	"dec":      func(i int) int { return i - 1 },
	"upper":    strings.ToUpper,
	"lower":    strings.ToLower,
	"title":    titulo,
	"miles":    miles,
	"round":    round,
	"tojson":   tojson,
	"str":      func(v any) string { return fmt.Sprintf("%v", v) },
	"toFloat":  func(v any) float64 { f, _ := aNum(v); return f },
	"toInt":    func(v any) int64 { f, _ := aNum(v); return int64(f) },
	"safeHTML": safeHTML,
}

// initTemplates parsea todas las plantillas .html de la carpeta templates.
func initTemplates() {
	_ = os.MkdirAll(templatesDir, 0o755)
	_ = os.MkdirAll(staticDir, 0o755)
	if err := cargarPlantillas(); err != nil {
		fmt.Printf("Error cargando plantillas: %v\n", err)
	}
}

func cargarPlantillas() error {
	t, err := template.New("mysys").Funcs(funcMap).Option("missingkey=zero").ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return err
	}
	tplMu.Lock()
	tpl = t
	tplModMax = modMaxPlantillas()
	tplMu.Unlock()
	return nil
}

// modMaxPlantillas devuelve la fecha de modificación más reciente de las plantillas.
func modMaxPlantillas() time.Time {
	var max time.Time
	entries, _ := os.ReadDir(templatesDir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(max) {
			max = info.ModTime()
		}
	}
	return max
}

// recargarSiCambio re-parsea las plantillas si alguna cambió en disco.
func recargarSiCambio() {
	if modMaxPlantillas().After(tplModMax) {
		if err := cargarPlantillas(); err != nil {
			fmt.Printf("Error recargando plantillas: %v\n", err)
		}
	}
}

// render ejecuta una plantilla Go con el contexto dado.
func render(w http.ResponseWriter, r *http.Request, nombre string, ctx map[string]any) {
	recargarSiCambio()
	if ctx == nil {
		ctx = map[string]any{}
	}
	queryParams := map[string]string{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			queryParams[k] = v[0]
		}
	}
	ctx["request"] = map[string]any{
		"url":          map[string]any{"path": r.URL.Path, "port": puertoDe(r)},
		"query_params": queryParams,
	}
	tplMu.Lock()
	t := tpl
	tplMu.Unlock()
	if t == nil {
		textoPlano(w, http.StatusInternalServerError, "Plantillas no cargadas.")
		return
	}
	plantilla := t.Lookup(nombre)
	if plantilla == nil {
		textoPlano(w, http.StatusInternalServerError,
			fmt.Sprintf("No se encontró la plantilla %q en %s.", nombre, templatesDir))
		return
	}
	var buf bytes.Buffer
	if err := plantilla.Execute(&buf, ctx); err != nil {
		textoPlano(w, http.StatusInternalServerError,
			fmt.Sprintf("Error ejecutando la plantilla %q:\n\n%s", nombre, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func puertoDe(r *http.Request) int {
	host := r.Host
	if i := strings.LastIndex(host, ":"); i >= 0 {
		var p int
		if _, err := fmt.Sscanf(host[i+1:], "%d", &p); err == nil {
			return p
		}
	}
	if r.TLS != nil {
		return 443
	}
	return 80
}

func textoPlano(w http.ResponseWriter, codigo int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(codigo)
	fmt.Fprintln(w, msg)
}

// renderError muestra una página de error (404.html, 500.html, 405.html) o
// texto plano si la plantilla no existe.
func renderError(w http.ResponseWriter, r *http.Request, codigo int, detalle string) {
	plantillas := map[int]string{404: "404.html", 500: "500.html", 405: "405.html"}
	nombre := plantillas[codigo]
	recargarSiCambio()
	tplMu.Lock()
	t := tpl
	tplMu.Unlock()
	if t != nil {
		if plantilla := t.Lookup(nombre); plantilla != nil {
			var buf bytes.Buffer
			if err := plantilla.Execute(&buf, map[string]any{
				"request": map[string]any{"url": map[string]any{"path": r.URL.Path}},
				"detail":  detalle,
			}); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(codigo)
				_, _ = w.Write(buf.Bytes())
				return
			}
		}
	}
	textoPlano(w, codigo, fmt.Sprintf("%d - %s", codigo, http.StatusText(codigo)))
}

// ── Sesiones ────────────────────────────────────────────────────────────────

func sesionDe(r *http.Request) *sessions.Session {
	s, _ := store.Get(r, nombreCookieSesion)
	return s
}

func sesionUsuario(r *http.Request) string {
	if v, ok := sesionDe(r).Values["username"].(string); ok {
		return v
	}
	return ""
}

func sesionRol(r *http.Request) string {
	if v, ok := sesionDe(r).Values["rol"].(string); ok {
		return v
	}
	return ""
}

// jsonResponde escribe una respuesta JSON.
func jsonResponde(w http.ResponseWriter, codigo int, datos any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(datos)
}

// jsonError equivale a raise HTTPException(status_code=..., detail=...).
func jsonError(w http.ResponseWriter, codigo int, detalle string) {
	jsonResponde(w, codigo, map[string]any{"detail": detalle})
}

// redirigir equivale a RedirectResponse(url=..., status_code=303).
func redirigir(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// ── Utilidades varias ───────────────────────────────────────────────────────

func redondearMonto(v float64) float64 { return database.RedondearMonto(v) }

// obtenerIPLocal obtiene la IP local de la máquina (igual que mi_ip.py).
func obtenerIPLocal() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		fmt.Printf("Error al obtener la IP local: %v\n", err)
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// formatoBytes formatea un tamaño en bytes (1024-based, igual que Python).
func formatoBytes(b float64) string {
	if b == 0 {
		return "0 B"
	}
	k := 1024.0
	sizes := []string{"B", "KB", "MB", "GB"}
	i := 0
	for b >= k && i < len(sizes)-1 {
		b /= k
		i++
	}
	return fmt.Sprintf("%.2f %s", b, sizes[i])
}

// iconoArchivo devuelve (clase, icono FontAwesome) según el tipo de archivo.
func iconoArchivo(mime, nombre string) (string, string) {
	ext := ""
	if i := strings.LastIndex(nombre, "."); i >= 0 {
		ext = strings.ToLower(nombre[i+1:])
	}
	contiene := strings.Contains
	switch {
	case contiene(mime, "pdf") || ext == "pdf":
		return "pdf", "fa-file-pdf"
	case contiene(mime, "word") || ext == "doc" || ext == "docx":
		return "doc", "fa-file-word"
	case contiene(mime, "excel") || ext == "xls" || ext == "xlsx":
		return "xls", "fa-file-excel"
	case contiene(mime, "image") || ext == "png" || ext == "jpg" || ext == "jpeg" || ext == "gif" || ext == "webp" || ext == "bmp":
		return "img", "fa-file-image"
	case contiene(mime, "zip") || ext == "zip" || ext == "rar" || ext == "7z" || ext == "tar":
		return "zip", "fa-file-zipper"
	}
	return "gen", "fa-file"
}

func ahoraTexto() string { return time.Now().Format("2006-01-02 15:04:05") }

// ── Presencia en línea (registro de actividad HTTP) ─────────────────────────

var (
	usuariosActividad   = map[string]time.Time{}
	usuariosActividadMu sync.Mutex
)

const minutosParaConsiderarDesconectado = 0

func marcarActividad(username string) {
	usuariosActividadMu.Lock()
	usuariosActividad[username] = time.Now()
	usuariosActividadMu.Unlock()
}

func quitarActividad(username string) {
	usuariosActividadMu.Lock()
	delete(usuariosActividad, username)
	usuariosActividadMu.Unlock()
}

func ultimaActividad(username string) (time.Time, bool) {
	usuariosActividadMu.Lock()
	defer usuariosActividadMu.Unlock()
	t, ok := usuariosActividad[username]
	return t, ok
}

// middlewareActividad registra la última actividad HTTP del usuario logueado.
func middlewareActividad(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if username := sesionUsuario(r); username != "" {
			marcarActividad(username)
		}
	})
}

// recover500 captura pánicos y muestra 500.html (equivalente al handler 500 de FastAPI).
func recover500(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Printf("error interno: %v\n", rec)
				renderError(w, r, http.StatusInternalServerError, fmt.Sprint(rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Columna define el mapeo clave → título conservando el orden (para Excel/PDF).
type Columna struct {
	Clave  string
	Titulo string
}
