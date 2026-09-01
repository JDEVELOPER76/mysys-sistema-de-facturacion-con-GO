package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"mysys/database"
)

// Instancias globales de la capa de datos (mismo orden de creación que Python,
// para que users.db exista antes de que ventas lo adjunte).
var (
	loginDB     *database.UserDB
	empleadoDB  *database.EmpleadoDB
	dbProductos *database.ProductoDB
	clienteDB   *database.ClienteDB
	ventasDB    *database.VentaDB
	auditoriaDB *database.AuditoriaDB
	chatDB      *database.ChatDB
	archivosDB  *database.ArchivosDB
)

func initDBs() {
	loginDB = database.NewUserDB()
	empleadoDB = database.NewEmpleadoDB()
	dbProductos = database.NewProductoDB()
	clienteDB = database.NewClienteDB()
	ventasDB = database.NewVentaDB()
	auditoriaDB = database.NewAuditoriaDB()
	chatDB = database.NewChatDB()
	archivosDB = database.NewArchivosDB()
}

// obtenerFotoUsuario devuelve la foto de perfil del usuario, o nil si no tiene
// o si el archivo ya no existe en disco (igual que la versión Python).
func obtenerFotoUsuario(username string) any {
	if username == "" {
		return nil
	}
	perfil := loginDB.ObtenerPerfil(username)
	if perfil == nil {
		return nil
	}
	foto, _ := perfil["foto"].(string)
	if foto == "" {
		return nil
	}
	if strings.HasPrefix(foto, "/static/") {
		archivo := filepath.Join(staticDir, strings.TrimPrefix(foto, "/static/"))
		if info, err := os.Stat(archivo); err == nil && !info.IsDir() {
			return foto
		}
		return nil
	}
	return foto
}

// nuevoServidor construye el router principal (puerto 8000).
func nuevoServidor() http.Handler {
	r := chi.NewRouter()

	r.Use(recover500)
	r.Use(middlewareActividad)

	// Páginas de error personalizadas (404/405)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		renderError(w, req, http.StatusNotFound, "")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		renderError(w, req, http.StatusMethodNotAllowed, "Método no permitido")
	})

	// Archivos estáticos
	fileServer := http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir)))
	r.Handle("/static/*", fileServer)

	// ── Autenticación y registro ──
	r.Get("/", vistaLogin)
	r.Get("/nuevo_registro", vistaNuevoRegistro)
	r.Post("/api/registrar_usuario", registrarUsuarioWeb)
	r.Post("/login", loginHandler)
	r.Get("/logout", logoutHandler)

	// ── Vistas de administración ──
	r.Get("/admin", vistaAdmin)
	r.Get("/admin/ventas", vistaAdminVentas)
	r.Get("/admin/productos", vistaProductos)
	r.Get("/admin/clientes", vistaClientes)
	r.Get("/admin/auditoria", vistaAuditoria)
	r.Get("/admin/usuarios", vistaAdminUsuarios)
	r.Get("/admin/estadistica", vistaEstadisticas)
	r.Get("/admin/en_linea", vistaUsuariosEnLinea)
	r.Get("/admin/archivos", vistaArchivos)
	r.Get("/admin/chats", vistaChatsAdmin)
	r.Get("/admin/reportes", vistaReportes)

	// ── Operaciones de administración ──
	r.Post("/admin/productos/nuevo", apiAgregarProducto)
	r.Post("/admin/productos/editar/{producto_id}", apiEditarProducto)
	r.Post("/admin/clientes/nuevo", apiAgregarCliente)
	r.Post("/admin/clientes/eliminar/{cliente_id}", apiEliminarCliente)
	r.Post("/admin/usuarios/nuevo", apiAgregarUsuario)
	r.Post("/admin/usuarios/cambiar_password/{username}", apiCambiarPassword)
	r.Post("/admin/usuarios/eliminar/{username}", apiEliminarUsuario)

	// ── Gestión de archivos ──
	r.Post("/admin/archivos/subir", apiSubirArchivo)
	r.Get("/admin/archivos/descargar/{archivo_id}", descargarArchivo)
	r.Post("/admin/archivos/eliminar/{archivo_id}", eliminarArchivo)

	// ── Vistas de empleado ──
	r.Get("/user/vender", vistaPanelVender)
	r.Get("/user/sala", vistaSalaEmpleado)
	r.Get("/user/chats", vistaChatsEmpleado)
	r.Get("/code", vistaScanner)

	// ── API de ventas y clientes ──
	r.Post("/api/vender", apiProcesarVenta)
	r.Get("/api/clientes/buscar", buscarClientes)
	r.Get("/api/clientes/rapido", metodoNoPermitido) // solo POST
	r.Post("/api/clientes/rapido", apiCrearClienteRapido)
	r.Get("/api/clientes/identificacion/{identificacion}", obtenerClientePorIdentificacion)
	r.Get("/api/clientes/{cliente_id}", obtenerClientePorID)

	// ── Escáner de códigos de barras ──
	r.Post("/api/scanner_login", scannerLogin)
	r.Post("/api/scanner_verify", scannerVerify)
	r.Post("/api/scanner_logout", scannerLogout)
	r.Post("/api/transmitir_escaneo", transmitirEscaneo)
	r.Get("/api/verificar_lecturas", verificarLecturas)
	r.Post("/api/leer_codigo", escanearNativo)
	r.Get("/api/scanner_status", scannerStatus)

	// ── Estadísticas ──
	r.Get("/api/estadisticas", apiEstadisticas)

	// ── En línea ──
	r.Get("/api/usuarios/en_linea", apiUsuariosEnLinea)

	// ── Chats privados ──
	r.Get("/api/chats/privados", apiChatsPrivados)

	// ── Perfil ──
	r.Get("/api/perfil", obtenerMiPerfil)
	r.Post("/api/perfil/editar", editarMiPerfil)
	r.Post("/api/perfil/foto", subirFotoPerfil)

	// ── Reportes (API + exportación) ──
	r.Get("/api/reportes/ventas", reporteVentas)
	r.Get("/api/reportes/auditoria", reporteAuditoria)
	r.Get("/api/reportes/productos", reporteProductos)
	r.Get("/api/reportes/clientes", reporteClientes)
	r.Get("/api/reportes/empleados", reporteEmpleados)
	r.Get("/api/reportes/exportar/ventas", exportarVentasExcel)
	r.Get("/api/reportes/exportar/auditoria", exportarAuditoriaExcel)
	r.Get("/api/reportes/exportar/productos", exportarProductosExcel)
	r.Get("/api/reportes/exportar/clientes", exportarClientesExcel)
	r.Get("/api/reportes/exportar/empleados", exportarEmpleadosExcel)
	r.Get("/api/reportes/exportar/pdf", exportarReportePDF)

	// ── WebSockets ──
	r.Get("/ws/presencia", wsPresencia)
	r.Get("/ws/en_linea", wsEnLinea)
	r.Get("/ws/archivos", wsArchivos)
	r.Get("/ws/chats", wsChats)

	return r
}

func metodoNoPermitido(w http.ResponseWriter, r *http.Request) {
	renderError(w, r, http.StatusMethodNotAllowed, "Método no permitido")
}

// requiereAdmin valida sesión + rol admin; devuelve "" si falla (ya respondió).
func requiereAdmin(w http.ResponseWriter, r *http.Request) string {
	username := sesionUsuario(r)
	rol := sesionRol(r)
	if username == "" || rol != "admin" {
		redirigir(w, r, "/?error=Acceso%20denegado")
		return ""
	}
	return username
}

// requiereAdminAPI es la variante JSON para endpoints API (403 en vez de redirect).
func requiereAdminAPI(w http.ResponseWriter, r *http.Request) string {
	username := sesionUsuario(r)
	rol := sesionRol(r)
	if username == "" || rol != "admin" {
		jsonError(w, http.StatusForbidden, "No autorizado")
		return ""
	}
	return username
}

// parsearRango interpreta desde/hasta (YYYY-MM-DD) como la versión Python:
// hasta se extiende al final del día (23:59:59). Definida en handlers_reportes.go.
