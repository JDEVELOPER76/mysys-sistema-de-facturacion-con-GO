package main

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ==========================================
//   PRESENCIA EN TIEMPO REAL (WebSocket)
// ==========================================

// ConnectionManager mantiene las conexiones WebSocket de presencia de cada
// usuario y las de los admins que miran el panel "En línea".
type ConnectionManager struct {
	mu                 sync.Mutex
	conexionesUsuarios map[string]map[*websocket.Conn]bool
	conexionesAdmin    map[*websocket.Conn]bool
}

func newConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		conexionesUsuarios: map[string]map[*websocket.Conn]bool{},
		conexionesAdmin:    map[*websocket.Conn]bool{},
	}
}

func (m *ConnectionManager) conectarUsuario(username string, ws *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conexionesUsuarios[username] == nil {
		m.conexionesUsuarios[username] = map[*websocket.Conn]bool{}
	}
	m.conexionesUsuarios[username][ws] = true
}

func (m *ConnectionManager) desconectarUsuario(username string, ws *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conexiones, ok := m.conexionesUsuarios[username]; ok {
		delete(conexiones, ws)
		if len(conexiones) == 0 {
			delete(m.conexionesUsuarios, username)
		}
	}
}

func (m *ConnectionManager) desconectarUsuarioCompleto(username string) []*websocket.Conn {
	m.mu.Lock()
	defer m.mu.Unlock()
	conexiones := []*websocket.Conn{}
	for ws := range m.conexionesUsuarios[username] {
		conexiones = append(conexiones, ws)
	}
	delete(m.conexionesUsuarios, username)
	return conexiones
}

func (m *ConnectionManager) conectarAdmin(ws *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conexionesAdmin[ws] = true
}

func (m *ConnectionManager) desconectarAdmin(ws *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conexionesAdmin, ws)
}

func (m *ConnectionManager) usuariosConectados() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for u := range m.conexionesUsuarios {
		out[u] = true
	}
	return out
}

// notificarAdmins empuja la lista actualizada a todos los admins conectados.
func (m *ConnectionManager) notificarAdmins() {
	payload := construirPayloadEnLinea()
	m.mu.Lock()
	admins := make([]*websocket.Conn, 0, len(m.conexionesAdmin))
	for ws := range m.conexionesAdmin {
		admins = append(admins, ws)
	}
	m.mu.Unlock()
	for _, ws := range admins {
		if err := ws.WriteJSON(payload); err != nil {
			m.desconectarAdmin(ws)
		}
	}
}

var manager = newConnectionManager()

func marcarUsuarioDesconectado(username string) {
	if username == "" {
		return
	}
	conexiones := manager.desconectarUsuarioCompleto(username)
	quitarActividad(username)
	manager.notificarAdmins()
	for _, ws := range conexiones {
		_ = ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		ws.Close()
	}
}

// usuariosActualmenteConectados une dos fuentes de verdad: los WebSocket de
// presencia abiertos ahora mismo y la actividad HTTP reciente (fallback).
func usuariosActualmenteConectados() map[string]bool {
	conectados := manager.usuariosConectados()
	ahora := time.Now()
	limite := time.Duration(minutosParaConsiderarDesconectado) * time.Minute
	usuariosActividadMu.Lock()
	for username, ultima := range usuariosActividad {
		if ahora.Sub(ultima) < limite {
			conectados[username] = true
		}
	}
	usuariosActividadMu.Unlock()
	return conectados
}

// construirPayloadEnLinea arma el JSON que se manda tanto por GET como por WebSocket.
func construirPayloadEnLinea() map[string]any {
	conectados := usuariosActualmenteConectados()
	enVivo := manager.usuariosConectados()

	todos := map[string]map[string]any{}
	for _, u := range loginDB.ObtenerUsuariosBasico() {
		if username, ok := u["username"].(string); ok {
			todos[username] = u
		}
	}

	lista := []map[string]any{}
	for username := range conectados {
		u := todos[username]
		nombre := username
		var puesto, tipo string
		var foto any
		if u != nil {
			if n, ok := u["nombre"].(string); ok && n != "" {
				nombre = n
			}
			puesto = str(u["puesto"])
			tipo = str(u["tipo"])
			foto = u["foto"]
		}
		var ultima any
		if t, ok := ultimaActividad(username); ok {
			ultima = t.Format("15:04:05")
		}
		lista = append(lista, map[string]any{
			"username":         username,
			"nombre":           nombre,
			"puesto":           puesto,
			"tipo":             tipo,
			"foto":             foto,
			"en_vivo":          enVivo[username],
			"ultima_actividad": ultima,
		})
	}
	sort.Slice(lista, func(i, j int) bool {
		return strings.ToLower(str(lista[i]["nombre"])) < strings.ToLower(str(lista[j]["nombre"]))
	})
	return map[string]any{"conectados": lista, "total": len(lista)}
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ── WebSocket: presencia ────────────────────────────────────────────────────

// wsPresencia lo abre cada usuario logueado mientras tiene la app abierta.
func wsPresencia(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		http.Error(w, "no autorizado", http.StatusUnauthorized)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	manager.conectarUsuario(username, ws)
	marcarActividad(username)
	manager.notificarAdmins()

	for {
		// El cliente manda un ping periódico para refrescar su última actividad.
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
		marcarActividad(username)
	}
	manager.desconectarUsuario(username, ws)
	manager.notificarAdmins()
}

// wsEnLinea lo consumen el panel de admin y la sala de empleados para recibir
// la lista de usuarios en línea en tiempo real, sin polling.
func wsEnLinea(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		http.Error(w, "no autorizado", http.StatusUnauthorized)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	manager.conectarAdmin(ws)
	_ = ws.WriteJSON(construirPayloadEnLinea())

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
	manager.desconectarAdmin(ws)
}

// ==========================================
//   WEBSOCKET DE ARCHIVOS
// ==========================================

type ArchivosConnectionManager struct {
	mu         sync.Mutex
	conexiones map[*websocket.Conn]bool
}

func newArchivosManager() *ArchivosConnectionManager {
	return &ArchivosConnectionManager{conexiones: map[*websocket.Conn]bool{}}
}

func (m *ArchivosConnectionManager) conectar(ws *websocket.Conn) {
	m.mu.Lock()
	m.conexiones[ws] = true
	m.mu.Unlock()
}

func (m *ArchivosConnectionManager) desconectar(ws *websocket.Conn) {
	m.mu.Lock()
	delete(m.conexiones, ws)
	m.mu.Unlock()
}

func (m *ArchivosConnectionManager) notificar(payload map[string]any) {
	m.mu.Lock()
	conexiones := make([]*websocket.Conn, 0, len(m.conexiones))
	for ws := range m.conexiones {
		conexiones = append(conexiones, ws)
	}
	m.mu.Unlock()
	for _, ws := range conexiones {
		if err := ws.WriteJSON(payload); err != nil {
			m.desconectar(ws)
		}
	}
}

var archivosManager = newArchivosManager()

func wsArchivos(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	if username == "" {
		http.Error(w, "no autorizado", http.StatusUnauthorized)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	archivosManager.conectar(ws)
	for {
		if _, _, err := ws.ReadMessage(); err != nil { // keep-alive
			break
		}
	}
	archivosManager.desconectar(ws)
}

// ==========================================
//   WEBSOCKET DE CHATS
// ==========================================

type ChatConnectionManager struct {
	mu    sync.Mutex
	salas map[string]map[*websocket.Conn]bool
}

func newChatManager() *ChatConnectionManager {
	return &ChatConnectionManager{salas: map[string]map[*websocket.Conn]bool{}}
}

func (m *ChatConnectionManager) entrar(sala string, ws *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.salas[sala] == nil {
		m.salas[sala] = map[*websocket.Conn]bool{}
	}
	m.salas[sala][ws] = true
}

func (m *ChatConnectionManager) salir(sala string, ws *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conexiones, ok := m.salas[sala]; ok {
		delete(conexiones, ws)
		if len(conexiones) == 0 {
			delete(m.salas, sala)
		}
	}
}

func (m *ChatConnectionManager) difundir(sala string, payload map[string]any) {
	m.mu.Lock()
	conexiones := make([]*websocket.Conn, 0, len(m.salas[sala]))
	for ws := range m.salas[sala] {
		conexiones = append(conexiones, ws)
	}
	m.mu.Unlock()
	for _, ws := range conexiones {
		if err := ws.WriteJSON(payload); err != nil {
			m.salir(sala, ws)
		}
	}
}

var chatManager = newChatManager()

// puedeEntrarSala aplica la regla de privacidad: las salas "dm:<a>:<b>" son
// privadas y solo admiten a esos dos usuarios; las demás son públicas.
func puedeEntrarSala(username, sala string) bool {
	if !strings.HasPrefix(sala, "dm:") {
		return true
	}
	partes := strings.Split(strings.TrimPrefix(sala, "dm:"), ":")
	if len(partes) != 2 || partes[0] == "" || partes[1] == "" || partes[0] == partes[1] {
		return false
	}
	return username == partes[0] || username == partes[1]
}

// sanitizarSala normaliza el nombre de sala (máx 80 caracteres).
func sanitizarSala(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 80 {
		s = string([]rune(s)[:80])
	}
	return s
}

func wsChats(w http.ResponseWriter, r *http.Request) {
	username := sesionUsuario(r)
	rol := sesionRol(r)
	if rol == "" {
		rol = "user"
	}
	if username == "" {
		http.Error(w, "no autorizado", http.StatusUnauthorized)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	// Una conexión puede estar unida a varias salas (general + privadas)
	salas := map[string]bool{}
	defer func() {
		for s := range salas {
			chatManager.salir(s, ws)
		}
	}()

	for {
		var mensaje map[string]any
		if err := ws.ReadJSON(&mensaje); err != nil {
			break
		}
		tipo, _ := mensaje["type"].(string)
		switch tipo {
		case "join":
			sala := sanitizarSala(str(mensaje["sala"]))
			if sala == "" {
				sala = "general"
			}
			if !puedeEntrarSala(username, sala) {
				_ = ws.WriteJSON(map[string]any{
					"type": "error", "sala": sala,
					"contenido": "No tienes acceso a esta conversación privada.",
				})
				continue
			}
			if !salas[sala] {
				chatManager.entrar(sala, ws)
				salas[sala] = true
			}
			// Historial con foto de perfil incluida
			mensajesRaw := chatDB.Listar(sala, 100)
			mensajesConFoto := make([]map[string]any, 0, len(mensajesRaw))
			for _, m := range mensajesRaw {
				m["foto"] = obtenerFotoUsuario(str(m["usuario"]))
				mensajesConFoto = append(mensajesConFoto, m)
			}
			_ = ws.WriteJSON(map[string]any{"type": "history", "sala": sala, "mensajes": mensajesConFoto})
		case "message":
			sala := sanitizarSala(str(mensaje["sala"]))
			if sala == "" || !salas[sala] || !puedeEntrarSala(username, sala) {
				continue
			}
			contenido, _ := mensaje["contenido"].(string)
			contenido = strings.TrimSpace(contenido)
			if len([]rune(contenido)) > 2000 {
				contenido = string([]rune(contenido)[:2000])
			}
			if contenido == "" {
				continue
			}
			nuevo := chatDB.Guardar(sala, username, rol, contenido)
			if nuevo == nil {
				continue
			}
			nuevo["foto"] = obtenerFotoUsuario(username)
			chatManager.difundir(sala, map[string]any{"type": "message", "sala": sala, "mensaje": nuevo})
		}
	}
}
