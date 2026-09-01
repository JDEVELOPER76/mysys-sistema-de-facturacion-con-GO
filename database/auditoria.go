package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"
)

// AuditoriaDB registra y consulta los eventos del sistema.
type AuditoriaDB struct {
	DB *sql.DB
}

// NewAuditoriaDB abre facturacion.db y asegura la tabla auditoria.
func NewAuditoriaDB(dbName ...string) *AuditoriaDB {
	name := "facturacion.db"
	if len(dbName) > 0 && dbName[0] != "" {
		name = dbName[0]
	}
	db, err := abrir(filepath.Join(CarpetaDatos, name))
	if err != nil {
		panic("no se pudo abrir facturacion.db: " + err.Error())
	}
	a := &AuditoriaDB{DB: db}
	a.crearTabla()
	return a
}

func (a *AuditoriaDB) crearTabla() {
	a.DB.Exec(`
		CREATE TABLE IF NOT EXISTS auditoria (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			usuario_id INTEGER,
			usuario TEXT,
			accion TEXT NOT NULL,
			tabla TEXT NOT NULL,
			registro_id INTEGER,
			detalles TEXT,
			fecha_hora TEXT,
			FOREIGN KEY (usuario_id) REFERENCES users(id)
		)`)
}

// Registrar guarda un evento de auditoría. Si fechaHora está vacía se usa la
// hora local del sistema con milisegundos (igual que la versión Python).
func (a *AuditoriaDB) Registrar(usuario string, usuarioID int64, accion, tabla string, registroID any, detalles string, fechaHora string) bool {
	if fechaHora == "" {
		ahora := time.Now()
		fechaHora = ahora.Format("2006-01-02 15:04:05") + fmt.Sprintf(".%03d", ahora.Nanosecond()/1e6)
	}
	if registroID == int64(0) {
		registroID = nil
	}
	var uid any = usuarioID
	if usuarioID == 0 {
		uid = nil
	}
	var det any = detalles
	if detalles == "" {
		det = nil
	}
	_, err := a.DB.Exec(`
		INSERT INTO auditoria (usuario_id, usuario, accion, tabla, registro_id, detalles, fecha_hora)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uid, usuario, accion, tabla, registroID, det, fechaHora)
	if err != nil {
		fmt.Printf("Error registrando auditoría: %v\n", err)
		return false
	}
	return true
}

const consultaLogs = `SELECT id, usuario_id, usuario, accion, tabla, registro_id, detalles, fecha_hora FROM auditoria`

// ObtenerLogs devuelve los últimos eventos de auditoría.
func (a *AuditoriaDB) ObtenerLogs(limite, offset int64) []map[string]any {
	if limite <= 0 {
		limite = 100
	}
	rows, err := a.DB.Query(consultaLogs+` ORDER BY fecha_hora DESC LIMIT ? OFFSET ?`, limite, offset)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ObtenerLogsUsuario devuelve los eventos de un usuario específico.
func (a *AuditoriaDB) ObtenerLogsUsuario(usuario string, limite int64) []map[string]any {
	if limite <= 0 {
		limite = 100
	}
	rows, err := a.DB.Query(consultaLogs+` WHERE usuario = ? ORDER BY fecha_hora DESC LIMIT ?`, usuario, limite)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ObtenerLogsTabla devuelve los eventos de una tabla específica.
func (a *AuditoriaDB) ObtenerLogsTabla(tabla string, limite int64) []map[string]any {
	if limite <= 0 {
		limite = 100
	}
	rows, err := a.DB.Query(consultaLogs+` WHERE tabla = ? ORDER BY fecha_hora DESC LIMIT ?`, tabla, limite)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ObtenerLogsPorPeriodo devuelve los eventos dentro de un rango de fechas.
func (a *AuditoriaDB) ObtenerLogsPorPeriodo(desde, hasta time.Time) []map[string]any {
	rows, err := a.DB.Query(consultaLogs+`
		WHERE fecha_hora BETWEEN ? AND ?
		ORDER BY fecha_hora DESC`,
		desde.Format("2006-01-02 15:04:05"), hasta.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil
	}
	return filas(rows)
}
