package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"mysys/database"
)

// ==========================================
//   API DE VENTAS
// ==========================================

// apiProcesarVenta registra una venta del punto de venta (POST /api/vender).
func apiProcesarVenta(w http.ResponseWriter, r *http.Request) {
	usuarioUsername := sesionUsuario(r)
	if usuarioUsername == "" {
		jsonError(w, http.StatusUnauthorized, "Sesión expirada. Inicie sesión nuevamente.")
		return
	}

	var data database.Venta
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}
	if len(data.Detalles) == 0 {
		jsonError(w, http.StatusBadRequest, "El carrito de compras está vacío.")
		return
	}

	empleadoID := loginDB.ObtenerIDUsuario(usuarioUsername)
	if empleadoID == 0 {
		jsonError(w, http.StatusBadRequest, "No se encontró el identificador del empleado.")
		return
	}

	// Calcular el total con IVA (igual que la versión Python)
	totalCalculado := 0.0
	for _, item := range data.Detalles {
		precioConIVA := redondearMonto(item.PrecioUnitario * (1 + item.IVA/100))
		totalCalculado += redondearMonto(float64(item.Cantidad) * precioConIVA)
	}
	total := redondearMonto(totalCalculado)

	metodo := data.MetodoPago
	if metodo == "" {
		metodo = "Efectivo"
	}
	nuevaVenta := database.Venta{
		ClienteID:  data.ClienteID,
		EmpleadoID: &empleadoID,
		Total:      &total,
		MetodoPago: metodo,
		Detalles:   data.Detalles,
	}

	resultado := ventasDB.RegistrarVenta(nuevaVenta, "", "")
	if ok, _ := resultado["success"].(bool); !ok {
		jsonError(w, http.StatusInternalServerError, database.AStr(resultado["error"]))
		return
	}

	ventaID := database.AInt(resultado["venta_id"])
	auditoriaDB.Registrar(usuarioUsername, empleadoID, "INSERT", "ventas", ventaID,
		fmt.Sprintf("Venta #%d registrada. Total cobrado con IVA: $%.2f. Método: %s.",
			ventaID, redondearMonto(totalCalculado), metodo),
		ahoraTexto())

	jsonResponde(w, http.StatusOK, map[string]any{"success": true, "venta_id": ventaID})
}

// ==========================================
//   API DE CLIENTES
// ==========================================

// buscarClientes autocompletado de clientes (GET /api/clientes/buscar?termino=).
func buscarClientes(w http.ResponseWriter, r *http.Request) {
	termino := r.URL.Query().Get("termino")
	if termino == "" || len(termino) < 2 {
		jsonResponde(w, http.StatusOK, map[string]any{"clientes": []any{}})
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{"clientes": clienteDB.BuscarClientes(termino)})
}

func obtenerClientePorID(w http.ResponseWriter, r *http.Request) {
	clienteID, _ := strconv.ParseInt(chi.URLParam(r, "cliente_id"), 10, 64)
	cliente := clienteDB.ObtenerClientePorID(clienteID)
	if cliente == nil {
		jsonError(w, http.StatusNotFound, "Cliente no encontrado")
		return
	}
	jsonResponde(w, http.StatusOK, cliente)
}

func obtenerClientePorIdentificacion(w http.ResponseWriter, r *http.Request) {
	identificacion := chi.URLParam(r, "identificacion")
	cliente := clienteDB.ObtenerClientePorIdentificacion(identificacion)
	if cliente == nil {
		jsonError(w, http.StatusNotFound, "Cliente no encontrado")
		return
	}
	jsonResponde(w, http.StatusOK, cliente)
}

// apiCrearClienteRapido crea un cliente desde el punto de venta (JSON).
func apiCrearClienteRapido(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}
	nombre, _ := data["nombre"].(string)
	if strings.TrimSpace(nombre) == "" {
		jsonError(w, http.StatusBadRequest, "El nombre es obligatorio")
		return
	}
	tipoID, _ := data["tipo_identificacion"].(string)
	if tipoID == "" {
		tipoID = "cedula"
	}
	getStr := func(k string) *string {
		if v, ok := data[k].(string); ok {
			return strONil(v)
		}
		return nil
	}
	resultado := clienteDB.AgregarCliente(strings.TrimSpace(nombre), tipoID,
		getStr("identificacion"), getStr("direccion"), getStr("telefono"), getStr("email"))
	if !resultado.Success {
		jsonError(w, http.StatusBadRequest, resultado.Error)
		return
	}
	cliente := clienteDB.ObtenerClientePorID(resultado.ID)
	jsonResponde(w, http.StatusOK, map[string]any{"success": true, "cliente": cliente})
}

// ==========================================
//   CONVERSACIONES PRIVADAS (1 a 1)
// ==========================================

// otroUsuarioDeSala devuelve el otro participante de una sala "dm:a:b".
func otroUsuarioDeSala(sala, yo string) string {
	if !strings.HasPrefix(sala, "dm:") {
		return ""
	}
	for _, p := range strings.Split(strings.TrimPrefix(sala, "dm:"), ":") {
		if p != yo {
			return p
		}
	}
	return ""
}

// listarPrivados arma la lista de conversaciones privadas del usuario con
// los datos del otro participante y el último mensaje.
func listarPrivados(username string) []map[string]any {
	filas := chatDB.ObtenerPrivadosDe(username)
	out := make([]map[string]any, 0, len(filas))
	for _, f := range filas {
		sala := database.AStr(f["sala"])
		otro := otroUsuarioDeSala(sala, username)
		if otro == "" {
			continue
		}
		perfil := loginDB.ObtenerPerfil(otro)
		nombre := otro
		if perfil != nil {
			if n := database.AStr(perfil["nombre"]); n != "" && n != "nulo" {
				nombre = n
			}
		}
		out = append(out, map[string]any{
			"sala":           sala,
			"otro_username":  otro,
			"otro_nombre":    nombre,
			"otro_foto":      obtenerFotoUsuario(otro),
			"ultimo_mensaje": database.AStr(f["contenido"]),
			"ultimo_de":      database.AStr(f["usuario"]),
			"enviado_en":     database.AStr(f["enviado_en"]),
		})
	}
	return out
}

// apiChatsPrivados devuelve las conversaciones privadas del usuario logueado.
func apiChatsPrivados(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		jsonError(w, http.StatusUnauthorized, "No autorizado")
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{"conversaciones": listarPrivados(username)})
}

// ==========================================
//   ESCÁNER DE CÓDIGOS DE BARRAS
// ==========================================

type scannerSesion struct {
	Username     string
	CreatedAt    time.Time
	LastActivity time.Time
	ExpiresAt    time.Time
}

var (
	scannerSessions   = map[string]*scannerSesion{}
	scannerSessionsMu sync.Mutex

	lecturasPendientes   = map[string][]string{}
	lecturasPendientesMu sync.Mutex
)

// limpiarSesionesAntiguas borra sesiones de escáner con más de 8 h sin actividad.
func limpiarSesionesAntiguas() {
	ahora := time.Now()
	scannerSessionsMu.Lock()
	for id, s := range scannerSessions {
		if ahora.Sub(s.LastActivity) > 8*time.Hour {
			delete(scannerSessions, id)
		}
	}
	scannerSessionsMu.Unlock()
}

func scannerLogin(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}
	username, _ := data["username"].(string)
	password, _ := data["password"].(string)
	if username == "" || password == "" {
		jsonError(w, http.StatusBadRequest, "Usuario y contraseña requeridos")
		return
	}

	if loginDB.VerificarUsuario(username, password) {
		sessionID := tokenURLSafe(32)
		ahora := time.Now()
		scannerSessionsMu.Lock()
		scannerSessions[sessionID] = &scannerSesion{
			Username:     username,
			CreatedAt:    ahora,
			LastActivity: ahora,
			ExpiresAt:    ahora.Add(8 * time.Hour),
		}
		scannerSessionsMu.Unlock()

		usuarioID := loginDB.ObtenerIDUsuario(username)
		auditoriaDB.Registrar(username, usuarioID, "SCANNER_LOGIN", "scanner_sessions", nil,
			"Inicio de sesión en el escáner de barras.", ahoraTexto())

		jsonResponde(w, http.StatusOK, map[string]any{
			"success": true, "session_id": sessionID, "username": username,
		})
		return
	}
	jsonError(w, http.StatusUnauthorized, "Credenciales inválidas")
}

func scannerVerify(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonResponde(w, http.StatusOK, map[string]any{"valid": false})
		return
	}
	username, _ := data["username"].(string)
	sessionID, _ := data["session_id"].(string)
	if username == "" || sessionID == "" {
		jsonResponde(w, http.StatusOK, map[string]any{"valid": false})
		return
	}

	scannerSessionsMu.Lock()
	s, ok := scannerSessions[sessionID]
	if ok && s.Username == username {
		s.LastActivity = time.Now()
		scannerSessionsMu.Unlock()
		jsonResponde(w, http.StatusOK, map[string]any{"valid": true})
		return
	}
	scannerSessionsMu.Unlock()
	jsonResponde(w, http.StatusOK, map[string]any{"valid": false})
}

// transmitirEscaneo recibe un código desde scanner.html y lo guarda para su usuario.
func transmitirEscaneo(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}
	codigo, _ := data["codigo_barras"].(string)
	username, _ := data["username"].(string)
	if codigo == "" {
		jsonError(w, http.StatusBadRequest, "No se proporcionó un código de barras.")
		return
	}
	if username == "" {
		jsonError(w, http.StatusBadRequest, "No se proporcionó el usuario del escáner.")
		return
	}

	lecturasPendientesMu.Lock()
	lecturasPendientes[username] = append(lecturasPendientes[username], codigo)
	lecturasPendientesMu.Unlock()

	jsonResponde(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Código transmitido exitosamente a la sesión de %s", username),
	})
}

// verificarLecturas lo consulta la terminal de ventas de la PC (polling).
func verificarLecturas(w http.ResponseWriter, r *http.Request) {
	usuarioPC := sesionUsuario(r)
	if usuarioPC == "" {
		jsonResponde(w, http.StatusOK, map[string]any{"conectado": false, "usuario": nil, "codigo": nil})
		return
	}

	ahora := time.Now()
	escanerConectado := false
	scannerSessionsMu.Lock()
	for _, s := range scannerSessions {
		if s.Username != usuarioPC {
			continue
		}
		expiresAt := s.ExpiresAt
		if expiresAt.IsZero() && !s.CreatedAt.IsZero() {
			expiresAt = s.CreatedAt.Add(8 * time.Hour)
		}
		if !expiresAt.IsZero() && ahora.Before(expiresAt) {
			escanerConectado = true
			break
		}
	}
	scannerSessionsMu.Unlock()

	var codigo any
	lecturasPendientesMu.Lock()
	if lista := lecturasPendientes[usuarioPC]; len(lista) > 0 {
		codigo = lista[0]
		lecturasPendientes[usuarioPC] = lista[1:]
	}
	lecturasPendientesMu.Unlock()

	jsonResponde(w, http.StatusOK, map[string]any{
		"conectado": escanerConectado,
		"usuario":   usuarioPC,
		"codigo":    codigo,
	})
}

func scannerLogout(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}
	username, _ := data["username"].(string)

	scannerSessionsMu.Lock()
	for id, s := range scannerSessions {
		if s.Username == username {
			delete(scannerSessions, id)
		}
	}
	scannerSessionsMu.Unlock()

	jsonResponde(w, http.StatusOK, map[string]any{"success": true})
}

// escanearNativo lee un código de barras y devuelve el producto (POST /api/leer_codigo).
func escanearNativo(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}
	codigoBarras, _ := data["codigo_barras"].(string)
	usuarioEscaner, _ := data["usuario"].(string)
	if codigoBarras == "" || usuarioEscaner == "" {
		jsonError(w, http.StatusBadRequest, "Faltan datos: código de barras o usuario.")
		return
	}

	// Validar que el usuario del escáner tiene una sesión activa
	sesionValida := false
	ahora := time.Now()
	scannerSessionsMu.Lock()
	for _, s := range scannerSessions {
		expiresAt := s.ExpiresAt
		if expiresAt.IsZero() && !s.CreatedAt.IsZero() {
			expiresAt = s.CreatedAt.Add(8 * time.Hour)
		}
		if s.Username == usuarioEscaner && !expiresAt.IsZero() && ahora.Before(expiresAt) {
			sesionValida = true
			break
		}
	}
	scannerSessionsMu.Unlock()

	if !sesionValida {
		jsonError(w, http.StatusForbidden, "Sesión de escáner no válida o expirada.")
		return
	}

	producto := dbProductos.ObtenerProductoPorCodigo(codigoBarras)
	if producto == nil {
		jsonError(w, http.StatusNotFound, "Producto no encontrado en el inventario.")
		return
	}
	jsonResponde(w, http.StatusOK, producto)
}

// scannerStatus indica si hay un escáner activo y quién lo usa.
func scannerStatus(w http.ResponseWriter, r *http.Request) {
	ahora := time.Now()
	activos := []string{}
	scannerSessionsMu.Lock()
	for _, s := range scannerSessions {
		if ahora.Sub(s.LastActivity) < time.Minute {
			activos = append(activos, s.Username)
		}
	}
	scannerSessionsMu.Unlock()

	if len(activos) > 0 {
		jsonResponde(w, http.StatusOK, map[string]any{"active": true, "usuario": activos[0]})
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{"active": false, "usuario": nil})
}
