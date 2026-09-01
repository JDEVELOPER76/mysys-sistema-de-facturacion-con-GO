package database

import (
	"database/sql"
	"path/filepath"
	"time"
)

// ArchivosDB gestiona el registro de archivos subidos (mysys.db).
type ArchivosDB struct {
	DB *sql.DB
}

// NewArchivosDB abre mysys.db (junto a la carpeta del paquete database,
// igual que la versión Python) y asegura la tabla archivos.
func NewArchivosDB(dbPath ...string) *ArchivosDB {
	path := filepath.Join(CarpetaDBRoot, "mysys.db")
	if len(dbPath) > 0 && dbPath[0] != "" {
		path = dbPath[0]
	}
	db, err := abrir(path)
	if err != nil {
		panic("no se pudo abrir mysys.db: " + err.Error())
	}
	a := &ArchivosDB{DB: db}
	a.crearTabla()
	return a
}

func (a *ArchivosDB) crearTabla() {
	a.DB.Exec(`
		CREATE TABLE IF NOT EXISTS archivos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nombre_original TEXT NOT NULL,
			nombre_almacenado TEXT NOT NULL UNIQUE,
			tamaño_bytes INTEGER NOT NULL,
			mime_type TEXT,
			subido_por TEXT NOT NULL,
			descripcion TEXT,
			fecha_subida TEXT NOT NULL
		)`)
}

// Guardar registra un archivo subido y devuelve la fila creada.
func (a *ArchivosDB) Guardar(nombreOriginal, nombreAlmacenado string, tamanoBytes int64, mimeType, subidoPor, descripcion string) map[string]any {
	var desc any
	if descripcion != "" {
		desc = descripcion
	}
	res, err := a.DB.Exec(`
		INSERT INTO archivos (nombre_original, nombre_almacenado, tamaño_bytes,
		                      mime_type, subido_por, descripcion, fecha_subida)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		nombreOriginal, nombreAlmacenado, tamanoBytes, mimeType,
		subidoPor, desc, time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil
	}
	id, _ := res.LastInsertId()
	return a.ObtenerPorID(id)
}

// Listar devuelve los últimos archivos subidos.
func (a *ArchivosDB) Listar(limite int64) []map[string]any {
	if limite <= 0 {
		limite = 100
	}
	rows, err := a.DB.Query(`SELECT * FROM archivos ORDER BY fecha_subida DESC LIMIT ?`, limite)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ObtenerPorID busca un archivo por ID.
func (a *ArchivosDB) ObtenerPorID(archivoID int64) map[string]any {
	return fila(a.DB, `SELECT * FROM archivos WHERE id = ?`, archivoID)
}

// Eliminar borra el registro de un archivo.
func (a *ArchivosDB) Eliminar(archivoID int64) bool {
	res, err := a.DB.Exec(`DELETE FROM archivos WHERE id = ?`, archivoID)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// Contar devuelve el número total de archivos.
func (a *ArchivosDB) Contar() int64 {
	row := fila(a.DB, `SELECT COUNT(*) as total FROM archivos`)
	if row == nil {
		return 0
	}
	return AInt(row["total"])
}

// TamanoTotal devuelve la suma de bytes de todos los archivos.
func (a *ArchivosDB) TamanoTotal() int64 {
	row := fila(a.DB, `SELECT COALESCE(SUM(tamaño_bytes), 0) as total FROM archivos`)
	if row == nil {
		return 0
	}
	return AInt(row["total"])
}
