package main

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"mysys/database"
)

// ==========================================
//   AUTENTICACIÓN Y REGISTRO WEB
// ==========================================

func vistaLogin(w http.ResponseWriter, r *http.Request) {
	render(w, r, "index.html", map[string]any{
		"error": r.URL.Query().Get("error"),
	})
}

func vistaNuevoRegistro(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	render(w, r, "nuevo_registro.html", map[string]any{
		"error":   q.Get("error"),
		"success": q.Get("success"),
	})
}

// registrarUsuarioWeb procesa el formulario de registro de usuario (POST).
func registrarUsuarioWeb(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderError(w, r, http.StatusBadRequest, "Formulario inválido")
		return
	}
	nombre := r.FormValue("nombre")
	puesto := r.FormValue("puesto")
	username := r.FormValue("username")
	password := r.FormValue("password")
	tipo := r.FormValue("tipo")
	salario, _ := strconv.ParseFloat(r.FormValue("salario"), 64)

	nuevo := database.Empleado{
		User:    database.User{Username: username, Password: password, Tipo: tipo},
		Nombre:  nombre,
		Puesto:  puesto,
		Salario: salario,
	}
	if !empleadoDB.AgregarEmpleado(nuevo) {
		redirigir(w, r, "/nuevo_registro?error=El%20nombre%20de%20usuario%20ya%20existe")
		return
	}

	// Auditoría (best effort)
	auditoriaDB.Registrar(username, 0, "REGISTRO_WEB", "users", nil,
		fmt.Sprintf("Nuevo usuario registrado vía web: @%s", username), ahoraTexto())

	redirigir(w, r, "/nuevo_registro?success=Usuario%20registrado%20exitosamente")
}

// loginHandler valida credenciales y crea la sesión.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderError(w, r, http.StatusBadRequest, "Formulario inválido")
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	if loginDB.VerificarUsuario(username, password) {
		sesion := sesionDe(r)
		sesion.Values["username"] = username
		usuarioID := loginDB.ObtenerIDUsuario(username)
		horaLocal := ahoraTexto()

		if loginDB.EsAdmin(username) {
			sesion.Values["rol"] = "admin"
			auditoriaDB.Registrar(username, usuarioID, "LOGIN", "users", usuarioID,
				"Inicio de sesión exitoso como Administrador.", horaLocal)
			_ = sesion.Save(r, w)
			redirigir(w, r, "/admin")
		} else {
			sesion.Values["rol"] = "user"
			auditoriaDB.Registrar(username, usuarioID, "LOGIN", "users", usuarioID,
				"Inicio de sesión exitoso como Operario/Empleado.", horaLocal)
			_ = sesion.Save(r, w)
			redirigir(w, r, "/user/vender")
		}
		return
	}
	redirigir(w, r, "/?error=Credenciales%20inv%C3%A1lidas")
}

// logoutHandler cierra la sesión y marca al usuario como desconectado.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	rol := sesionRol(r)
	if username != "" {
		usuarioID := loginDB.ObtenerIDUsuario(username)
		auditoriaDB.Registrar(username, usuarioID, "LOGOUT", "users", usuarioID,
			fmt.Sprintf("Cierre de sesión exitoso. Rol: %s.", rol), ahoraTexto())
		marcarUsuarioDesconectado(username)
	}
	sesion := sesionDe(r)
	sesion.Options.MaxAge = -1 // borra la cookie
	_ = sesion.Save(r, w)
	redirigir(w, r, "/")
}

// ==========================================
//   VISTAS DE ADMINISTRACIÓN
// ==========================================

func vistaAdmin(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	render(w, r, "admin.html", map[string]any{
		"username":        username,
		"foto":            obtenerFotoUsuario(username),
		"total_hoy":       ventasDB.ObtenerTotalVentasHoy(),
		"total_historico": ventasDB.ObtenerTotalVentas(),
		"ultimas_ventas":  ventasDB.ListarUltimasVentas(10),
		"ventas_por_dia":  ventasDB.ObtenerVentasTotalesPorDia(),
	})
}

func vistaAdminVentas(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	ultimasVentas := ventasDB.ListarUltimasVentas(20)
	ventasConProductos := []map[string]any{}
	for _, venta := range ultimasVentas {
		detalle := ventasDB.ObtenerVentaConProductos(database.AInt(venta["id"]))
		if len(detalle) > 0 {
			// La plantilla muestra la fecha con fecha_completa_dt.Format(...)
			if fc, ok := detalle["fecha_completa"].(string); ok {
				for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", time.RFC3339} {
					if t, err := time.ParseInLocation(layout, fc, time.Local); err == nil {
						detalle["fecha_completa_dt"] = t
						break
					}
				}
			}
			ventasConProductos = append(ventasConProductos, detalle)
		}
	}
	render(w, r, "admin_ventas.html", map[string]any{
		"username":             username,
		"foto":                 obtenerFotoUsuario(username),
		"ventas_con_productos": ventasConProductos,
	})
}

func vistaProductos(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	render(w, r, "admin_productos.html", map[string]any{
		"username":  username,
		"foto":      obtenerFotoUsuario(username),
		"productos": dbProductos.ListarProductos(100, 0),
	})
}

func vistaClientes(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	render(w, r, "admin_clientes.html", map[string]any{
		"username": username,
		"foto":     obtenerFotoUsuario(username),
		"clientes": clienteDB.ObtenerTodosLosClientes(),
	})
}

func vistaAuditoria(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	render(w, r, "admin_auditoria.html", map[string]any{
		"username":       username,
		"foto":           obtenerFotoUsuario(username),
		"logs_auditoria": auditoriaDB.ObtenerLogs(100, 0),
	})
}

func vistaAdminUsuarios(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	render(w, r, "admin_usuarios.html", map[string]any{
		"username": username,
		"foto":     obtenerFotoUsuario(username),
		"usuarios": empleadoDB.ObtenerUsuarios(),
	})
}

func vistaUsuariosEnLinea(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	render(w, r, "admin_en_linea.html", map[string]any{
		"username": username,
		"foto":     obtenerFotoUsuario(username),
	})
}

// listarEquipo devuelve todos los usuarios con su foto validada en disco,
// nombre visible y puesto, ordenados alfabéticamente (para el panel de chats).
func listarEquipo() []map[string]any {
	usuarios := loginDB.ObtenerUsuariosBasico()
	equipo := make([]map[string]any, 0, len(usuarios))
	for _, u := range usuarios {
		username := database.AStr(u["username"])
		nombre := database.AStr(u["nombre"])
		if nombre == "" || nombre == "nulo" {
			nombre = username
		}
		puesto := database.AStr(u["puesto"])
		if puesto == "nulo" {
			puesto = ""
		}
		equipo = append(equipo, map[string]any{
			"username":       username,
			"nombre_visible": nombre,
			"puesto":         puesto,
			"tipo":           database.AStr(u["tipo"]),
			"foto":           obtenerFotoUsuario(username),
		})
	}
	sort.Slice(equipo, func(i, j int) bool {
		return strings.ToLower(database.AStr(equipo[i]["nombre_visible"])) <
			strings.ToLower(database.AStr(equipo[j]["nombre_visible"]))
	})
	return equipo
}

func vistaChatsAdmin(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	render(w, r, "admin_chats.html", map[string]any{
		"username":        username,
		"rol":             sesionRol(r),
		"foto":            obtenerFotoUsuario(username),
		"usuarios_equipo": listarEquipo(),
		"chats_privados":  listarPrivados(username),
	})
}

func vistaChatsEmpleado(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		redirigir(w, r, "/?error=Sesión%20requerida")
		return
	}
	rol := sesionRol(r)
	if rol == "" {
		rol = "user"
	}
	render(w, r, "empleado_chat.html", map[string]any{
		"username":        username,
		"rol":             rol,
		"foto":            obtenerFotoUsuario(username),
		"usuarios_equipo": listarEquipo(),
		"chats_privados":  listarPrivados(username),
	})
}

func vistaReportes(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	render(w, r, "admin_reportes.html", map[string]any{
		"username": username,
		"foto":     obtenerFotoUsuario(username),
	})
}

func vistaScanner(w http.ResponseWriter, r *http.Request) {
	render(w, r, "scanner.html", map[string]any{})
}

// ==========================================
//   VISTAS DE EMPLEADO
// ==========================================

func vistaPanelVender(w http.ResponseWriter, r *http.Request) {
	usuarioActual := sesionUsuario(r)
	if usuarioActual == "" {
		redirigir(w, r, "/")
		return
	}
	render(w, r, "empleado_vender.html", map[string]any{
		"productos": dbProductos.ListarProductos(200, 0),
		"clientes":  clienteDB.ObtenerTodosLosClientes(),
		"usuario":   usuarioActual,
	})
}

func vistaSalaEmpleado(w http.ResponseWriter, r *http.Request) {
	usuarioActual := sesionUsuario(r)
	if usuarioActual == "" {
		redirigir(w, r, "/")
		return
	}
	perfil := loginDB.ObtenerPerfil(usuarioActual)
	if perfil == nil {
		jsonError(w, http.StatusNotFound, "No se encontró el perfil del usuario.")
		return
	}
	render(w, r, "empleado_sala.html", map[string]any{
		"usuario":     usuarioActual,
		"perfil":      perfil,
		"server_ip":   obtenerIPLocal(),
		"server_port": puertoDe(r),
	})
}

// ==========================================
//   PRODUCTOS (admin)
// ==========================================

// guardarImagenProducto guarda el archivo subido como <codigo_barras><ext>.
func guardarImagenProducto(r *http.Request, codigoBarras string) (string, error) {
	archivo, cabecera, err := r.FormFile("imagen_archivo")
	if err != nil || cabecera == nil || cabecera.Filename == "" {
		return "", nil // sin archivo
	}
	defer archivo.Close()

	extension := filepath.Ext(cabecera.Filename)
	nombreArchivo := codigoBarras + extension
	rutaGuardado := filepath.Join(CarpetaImagenes, nombreArchivo)

	contenido, err := io.ReadAll(archivo)
	if err != nil {
		return "", fmt.Errorf("Error al guardar la imagen: %w", err)
	}
	if err := escribirArchivo(rutaGuardado, contenido); err != nil {
		return "", fmt.Errorf("Error al guardar la imagen: %w", err)
	}
	return "/static/productos_img/" + nombreArchivo, nil
}

func apiAgregarProducto(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		username = "Desconocido"
	}
	if sesionUsuario(r) == "" || sesionRol(r) != "admin" {
		jsonError(w, http.StatusForbidden, "No autorizado")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jsonError(w, http.StatusBadRequest, "Formulario inválido")
		return
	}

	precio, _ := strconv.ParseFloat(r.FormValue("precio"), 64)
	iva, _ := strconv.ParseFloat(r.FormValue("iva"), 64)
	stock, _ := strconv.ParseInt(r.FormValue("stock"), 10, 64)
	codigoBarras := r.FormValue("codigo_barras")

	// Imagen: archivo subido o URL pegada
	finalImagePath, err := guardarImagenProducto(r, codigoBarras)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if finalImagePath == "" {
		if u := strings.TrimSpace(r.FormValue("imagen_url")); u != "" {
			finalImagePath = u
		}
	}

	nuevo := database.Producto{
		Nombre:       r.FormValue("nombre"),
		Descripcion:  r.FormValue("descripcion"),
		Precio:       precio,
		IVA:          iva,
		CodigoBarras: codigoBarras,
		Proveedor:    r.FormValue("proveedor"),
		Stock:        stock,
		Categoria:    r.FormValue("categoria"),
		ImagenURL:    finalImagePath,
	}

	resultado := dbProductos.AgregarProducto(nuevo)
	if !resultado.Success {
		jsonError(w, http.StatusBadRequest, resultado.Error)
		return
	}

	usuarioID := loginDB.ObtenerIDUsuario(username)
	auditoriaDB.Registrar(username, usuarioID, "INSERT", "productos", nil,
		fmt.Sprintf("Se agregó el producto '%s' con stock inicial de %d.", nuevo.Nombre, stock),
		ahoraTexto())

	redirigir(w, r, "/admin/productos")
}

func apiEditarProducto(w http.ResponseWriter, r *http.Request) {
	productoID, _ := strconv.ParseInt(chi.URLParam(r, "producto_id"), 10, 64)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jsonError(w, http.StatusBadRequest, "Formulario inválido")
		return
	}

	// Buscar el producto actual para conservar su imagen si no se sube otra.
	productoActual := dbProductos.ObtenerProducto(productoID)
	if productoActual == nil {
		jsonError(w, http.StatusNotFound, "Producto no encontrado en el sistema")
		return
	}
	finalImagePath := database.AStr(productoActual["imagen_url"])

	precio, _ := strconv.ParseFloat(r.FormValue("precio"), 64)
	iva, _ := strconv.ParseFloat(r.FormValue("iva"), 64)
	stock, _ := strconv.ParseInt(r.FormValue("stock"), 10, 64)
	codigoBarras := r.FormValue("codigo_barras")

	// Caso A: se subió un archivo nuevo; Caso B: se pegó una URL nueva.
	if nueva, err := guardarImagenProducto(r, codigoBarras); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	} else if nueva != "" {
		finalImagePath = nueva
	} else if u := strings.TrimSpace(r.FormValue("imagen_url")); u != "" {
		finalImagePath = u
	}

	actualizado := database.Producto{
		Nombre:       r.FormValue("nombre"),
		Descripcion:  r.FormValue("descripcion"),
		Precio:       precio,
		IVA:          iva,
		CodigoBarras: codigoBarras,
		Proveedor:    r.FormValue("proveedor"),
		Stock:        stock,
		Categoria:    r.FormValue("categoria"),
		ImagenURL:    finalImagePath,
	}
	resultado := dbProductos.ActualizarProducto(productoID, actualizado)
	if !resultado.Success {
		jsonError(w, http.StatusBadRequest, resultado.Error)
		return
	}
	redirigir(w, r, "/admin/productos")
}

// ==========================================
//   CLIENTES (admin, formularios)
// ==========================================

func strONil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func apiAgregarCliente(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderError(w, r, http.StatusBadRequest, "Formulario inválido")
		return
	}
	nombre := strings.TrimSpace(r.FormValue("nombre"))
	if nombre == "" {
		jsonError(w, http.StatusBadRequest, "El nombre del cliente es obligatorio")
		return
	}
	tipoID := r.FormValue("tipo_identificacion")
	if tipoID == "" {
		tipoID = "cedula"
	}
	resultado := clienteDB.AgregarCliente(nombre, tipoID,
		strONil(r.FormValue("identificacion")), strONil(r.FormValue("direccion")),
		strONil(r.FormValue("telefono")), strONil(r.FormValue("email")))
	if !resultado.Success {
		jsonError(w, http.StatusBadRequest, resultado.Error)
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{
		"success": true, "id": resultado.ID, "message": "Cliente agregado exitosamente",
	})
}

func apiEliminarCliente(w http.ResponseWriter, r *http.Request) {
	clienteID, _ := strconv.ParseInt(chi.URLParam(r, "cliente_id"), 10, 64)
	resultado := clienteDB.EliminarCliente(clienteID)
	if !resultado.Success {
		jsonError(w, http.StatusBadRequest, resultado.Error)
		return
	}
	redirigir(w, r, "/admin/clientes")
}

// ==========================================
//   USUARIOS (admin)
// ==========================================

func apiAgregarUsuario(w http.ResponseWriter, r *http.Request) {
	usuarioAdmin := sesionUsuario(r)
	if usuarioAdmin == "" {
		usuarioAdmin = "Desconocido"
	}
	if err := r.ParseForm(); err != nil {
		renderError(w, r, http.StatusBadRequest, "Formulario inválido")
		return
	}
	salario, _ := strconv.ParseFloat(r.FormValue("salario"), 64)
	tipo := r.FormValue("tipo")
	nuevo := database.Empleado{
		User: database.User{
			Username: r.FormValue("username"),
			Password: r.FormValue("password"),
			Tipo:     tipo,
		},
		Nombre:  r.FormValue("nombre"),
		Puesto:  r.FormValue("puesto"),
		Salario: salario,
	}
	if !empleadoDB.AgregarEmpleado(nuevo) {
		jsonError(w, http.StatusBadRequest, "El nombre de usuario ya existe. Elija otro.")
		return
	}
	usuarioID := loginDB.ObtenerIDUsuario(usuarioAdmin)
	auditoriaDB.Registrar(usuarioAdmin, usuarioID, "INSERT", "users", nil,
		fmt.Sprintf("Se creó un nuevo usuario con rol '%s': @%s (%s).",
			strings.ToUpper(tipo), nuevo.Username, nuevo.Nombre),
		ahoraTexto())
	redirigir(w, r, "/admin/usuarios")
}

func apiCambiarPassword(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	_ = r.ParseForm()
	empleadoDB.CambiarPassword(username, r.FormValue("nueva_clave"))
	redirigir(w, r, "/admin/usuarios")
}

func apiEliminarUsuario(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	empleadoDB.EliminarUsuario(username)
	redirigir(w, r, "/admin/usuarios")
}

// ==========================================
//   GESTIÓN DE ARCHIVOS
// ==========================================

func vistaArchivos(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}
	archivosRaw := archivosDB.Listar(200)
	archivos := []map[string]any{}
	usuariosUnicos := map[string]bool{}
	var tamanoTotal int64
	for _, a := range archivosRaw {
		tam := database.AInt(a["tamaño_bytes"])
		tamanoTotal += tam
		usuariosUnicos[database.AStr(a["subido_por"])] = true
		iconoCls, iconoIcon := iconoArchivo(database.AStr(a["mime_type"]), database.AStr(a["nombre_original"]))
		a["tamaño_formateado"] = formatoBytes(float64(tam))
		// Alias ASCII para las plantillas (las claves con ñ se evitan en templates)
		a["tamano_bytes"] = a["tamaño_bytes"]
		a["tamano_formateado"] = a["tamaño_formateado"]
		a["icono_cls"] = iconoCls
		a["icono_icon"] = iconoIcon
		archivos = append(archivos, a)
	}
	render(w, r, "admin_archivos.html", map[string]any{
		"username":                username,
		"foto":                    obtenerFotoUsuario(username),
		"archivos":                archivos,
		"tamano_total_formateado": formatoBytes(float64(tamanoTotal)),
		"total_usuarios_unicos":   len(usuariosUnicos),
	})
}

func apiSubirArchivo(w http.ResponseWriter, r *http.Request) {
	username := requiereAdminAPI(w, r)
	if username == "" {
		return
	}
	// Máx 50 MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		jsonError(w, http.StatusRequestEntityTooLarge, "El archivo excede los 50 MB permitidos")
		return
	}
	archivo, cabecera, err := r.FormFile("archivo")
	if err != nil || cabecera == nil || cabecera.Filename == "" {
		jsonError(w, http.StatusBadRequest, "No se proporcionó ningún archivo")
		return
	}
	defer archivo.Close()

	contenido, err := io.ReadAll(archivo)
	if err != nil {
		jsonError(w, http.StatusRequestEntityTooLarge, "El archivo excede los 50 MB permitidos")
		return
	}
	tamano := int64(len(contenido))
	if tamano > 50*1024*1024 {
		jsonError(w, http.StatusRequestEntityTooLarge, "El archivo excede los 50 MB permitidos")
		return
	}

	extension := strings.ToLower(filepath.Ext(cabecera.Filename))
	nombreSeguro := tokenHex(16) + extension
	rutaGuardado := filepath.Join(CarpetaArchivos, nombreSeguro)
	if err := escribirArchivo(rutaGuardado, contenido); err != nil {
		jsonError(w, http.StatusInternalServerError, "No se pudo guardar el archivo")
		return
	}

	mime := cabecera.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	nuevo := archivosDB.Guardar(cabecera.Filename, nombreSeguro, tamano, mime,
		username, strings.TrimSpace(r.FormValue("descripcion")))

	// Auditoría
	usuarioID := loginDB.ObtenerIDUsuario(username)
	var registroID any
	if nuevo != nil {
		registroID = nuevo["id"]
	}
	auditoriaDB.Registrar(username, usuarioID, "INSERT", "archivos", registroID,
		fmt.Sprintf("Subió archivo '%s' (%s).", cabecera.Filename, formatoBytes(float64(tamano))),
		ahoraTexto())

	// Notificar vía WebSocket a todos los clientes conectados
	if nuevo != nil {
		iconoCls, iconoIcon := iconoArchivo(mime, cabecera.Filename)
		archivoPayload := map[string]any{}
		for k, v := range nuevo {
			archivoPayload[k] = v
		}
		archivoPayload["tamaño_formateado"] = formatoBytes(float64(tamano))
		archivoPayload["icono_cls"] = iconoCls
		archivoPayload["icono_icon"] = iconoIcon
		archivosManager.notificar(map[string]any{"type": "nuevo_archivo", "archivo": archivoPayload})
	}

	jsonResponde(w, http.StatusOK, map[string]any{"success": true, "archivo": nuevo})
}

func descargarArchivo(w http.ResponseWriter, r *http.Request) {
	if sesionUsuario(r) == "" {
		jsonError(w, http.StatusUnauthorized, "No autorizado")
		return
	}
	archivoID, _ := strconv.ParseInt(chi.URLParam(r, "archivo_id"), 10, 64)
	archivo := archivosDB.ObtenerPorID(archivoID)
	if archivo == nil {
		jsonError(w, http.StatusNotFound, "Archivo no encontrado")
		return
	}
	ruta := filepath.Join(CarpetaArchivos, database.AStr(archivo["nombre_almacenado"]))
	if info, err := osStat(ruta); err != nil || info.IsDir() {
		jsonError(w, http.StatusNotFound, "Archivo no encontrado en disco")
		return
	}
	mime := database.AStr(archivo["mime_type"])
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(database.AStr(archivo["nombre_original"]), `"`, "")))
	http.ServeFile(w, r, ruta)
}

func eliminarArchivo(w http.ResponseWriter, r *http.Request) {
	username := requiereAdminAPI(w, r)
	if username == "" {
		return
	}
	archivoID, _ := strconv.ParseInt(chi.URLParam(r, "archivo_id"), 10, 64)
	archivo := archivosDB.ObtenerPorID(archivoID)
	if archivo == nil {
		jsonError(w, http.StatusNotFound, "Archivo no encontrado")
		return
	}

	// Eliminar del disco
	ruta := filepath.Join(CarpetaArchivos, database.AStr(archivo["nombre_almacenado"]))
	_ = osRemove(ruta)

	archivosDB.Eliminar(archivoID)

	// Auditoría
	usuarioID := loginDB.ObtenerIDUsuario(username)
	auditoriaDB.Registrar(username, usuarioID, "DELETE", "archivos", archivoID,
		fmt.Sprintf("Eliminó archivo '%s'.", database.AStr(archivo["nombre_original"])),
		ahoraTexto())

	// Notificar vía WebSocket
	archivosManager.notificar(map[string]any{"type": "archivo_eliminado", "id": archivoID})

	jsonResponde(w, http.StatusOK, map[string]any{"success": true})
}

// ── En línea (fallback HTTP) ────────────────────────────────────────────────

func apiUsuariosEnLinea(w http.ResponseWriter, r *http.Request) {
	if sesionUsuario(r) == "" {
		jsonError(w, http.StatusUnauthorized, "No autorizado")
		return
	}
	jsonResponde(w, http.StatusOK, construirPayloadEnLinea())
}

// ==========================================
//   PERFIL
// ==========================================

func obtenerMiPerfil(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		jsonError(w, http.StatusUnauthorized, "No autorizado")
		return
	}
	perfil := loginDB.ObtenerPerfil(username)
	if perfil == nil {
		jsonError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	}
	jsonResponde(w, http.StatusOK, perfil)
}

func editarMiPerfil(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		jsonError(w, http.StatusUnauthorized, "No autorizado")
		return
	}
	_ = r.ParseForm()
	nombre := strings.TrimSpace(r.FormValue("nombre"))
	puesto := strONil(r.FormValue("puesto"))
	loginDB.ActualizarDatosBasicos(username, &nombre, puesto)

	usuarioID := loginDB.ObtenerIDUsuario(username)
	auditoriaDB.Registrar(username, usuarioID, "UPDATE", "users", usuarioID,
		"El usuario actualizó los datos de su perfil.", ahoraTexto())

	jsonResponde(w, http.StatusOK, map[string]any{"success": true})
}

func subirFotoPerfil(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		jsonError(w, http.StatusUnauthorized, "No autorizado")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jsonError(w, http.StatusBadRequest, "Formulario inválido")
		return
	}
	archivo, cabecera, err := r.FormFile("imagen_archivo")
	if err != nil || cabecera == nil || cabecera.Filename == "" {
		jsonError(w, http.StatusBadRequest, "No se proporcionó ninguna imagen")
		return
	}
	defer archivo.Close()

	extension := strings.ToLower(filepath.Ext(cabecera.Filename))
	permitidas := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}
	if !permitidas[extension] {
		jsonError(w, http.StatusBadRequest, "Formato de imagen no permitido")
		return
	}

	nombreArchivo := username + extension
	rutaGuardado := filepath.Join(CarpetaPerfilImg, nombreArchivo)
	contenido, err := io.ReadAll(archivo)
	if err != nil || escribirArchivo(rutaGuardado, contenido) != nil {
		jsonError(w, http.StatusBadRequest, "Error al guardar la imagen")
		return
	}
	rutaFinal := "/static/perfil_img/" + nombreArchivo

	loginDB.ActualizarFoto(username, rutaFinal)

	usuarioID := loginDB.ObtenerIDUsuario(username)
	auditoriaDB.Registrar(username, usuarioID, "UPDATE", "users", usuarioID,
		"El usuario actualizó su foto de perfil.", ahoraTexto())

	jsonResponde(w, http.StatusOK, map[string]any{"success": true, "foto": rutaFinal})
}
