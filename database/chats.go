package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"time"
)

// ChatDB persistencia del sistema de mensajería de MySYS.
type ChatDB struct {
	DB *sql.DB
}

// NewChatDB abre chats.db y asegura el esquema.
func NewChatDB(dbName ...string) *ChatDB {
	name := "chats.db"
	if len(dbName) > 0 && dbName[0] != "" {
		name = dbName[0]
	}
	db, err := abrir(filepath.Join(CarpetaChats, name))
	if err != nil {
		panic("no se pudo abrir chats.db: " + err.Error())
	}
	c := &ChatDB{DB: db}
	c.crearTablas()
	return c
}

func (c *ChatDB) crearTablas() {
	c.DB.Exec(`
		CREATE TABLE IF NOT EXISTS mensajes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sala TEXT NOT NULL DEFAULT 'general',
			usuario TEXT NOT NULL,
			rol TEXT NOT NULL DEFAULT 'user',
			contenido TEXT NOT NULL,
			enviado_en TEXT NOT NULL
		)`)
	c.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_mensajes_sala_fecha ON mensajes(sala, id)`)
}

// Listar devuelve los últimos mensajes de una sala en orden cronológico.
func (c *ChatDB) Listar(sala string, limite int64) []map[string]any {
	if sala == "" {
		sala = "general"
	}
	if limite <= 0 {
		limite = 100
	}
	rows, err := c.DB.Query(`
		SELECT id, sala, usuario, rol, contenido, enviado_en
		FROM mensajes WHERE sala = ? ORDER BY id DESC LIMIT ?`, sala, limite)
	if err != nil {
		return nil
	}
	desc := filas(rows)
	// Invertir para orden cronológico ascendente (como reversed() en Python).
	out := make([]map[string]any, 0, len(desc))
	for i := len(desc) - 1; i >= 0; i-- {
		out = append(out, desc[i])
	}
	return out
}

// Guardar almacena un mensaje y lo devuelve con su id y fecha.
func (c *ChatDB) Guardar(sala, usuario, rol, contenido string) map[string]any {
	ahora := time.Now().Format("2006-01-02T15:04:05")
	res, err := c.DB.Exec(`
		INSERT INTO mensajes (sala, usuario, rol, contenido, enviado_en) VALUES (?, ?, ?, ?, ?)`,
		sala, usuario, rol, contenido, ahora)
	if err != nil {
		return nil
	}
	id, _ := res.LastInsertId()
	return map[string]any{
		"id": id, "sala": sala, "usuario": usuario, "rol": rol,
		"contenido": contenido, "enviado_en": ahora,
	}
}

// ObtenerPrivadosDe devuelve las conversaciones privadas (dm:a:b) en las que
// participa el usuario, con el último mensaje de cada una.
func (c *ChatDB) ObtenerPrivadosDe(username string) []map[string]any {
	// Escapar comodines LIKE en el username
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(username)
	likeInicio := "dm:" + esc + ":%"
	likeFin := "dm:%:" + esc
	rows, err := c.DB.Query(`
		SELECT m.sala, m.usuario, m.contenido, m.enviado_en
		FROM mensajes m
		JOIN (
			SELECT sala, MAX(id) AS max_id
			FROM mensajes
			WHERE sala LIKE ? ESCAPE '\' OR sala LIKE ? ESCAPE '\'
			GROUP BY sala
		) ult ON m.sala = ult.sala AND m.id = ult.max_id
		ORDER BY m.id DESC`, likeInicio, likeFin)
	if err != nil {
		return nil
	}
	return filas(rows)
}
