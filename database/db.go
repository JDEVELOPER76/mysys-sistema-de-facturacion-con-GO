// Package database contiene la capa de acceso a datos de MySYS (SQLite).
// Replica exactamente el esquema y las rutas de la versión Python, por lo que
// las bases de datos existentes siguen siendo compatibles.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Rutas base (equivalentes a las de la versión Python):
//
//	<raiz>/database/datos_facturacion/datos/facturacion.db
//	<raiz>/database/datos_facturacion/usuarios/users.db
//	<raiz>/database/datos_facturacion/chats/chats.db
//	<raiz>/database/mysys.db  (archivos)
var (
	Raiz          string
	CarpetaBase   string
	CarpetaDatos  string
	CarpetaUsers  string
	CarpetaChats  string
	CarpetaDBRoot string
)

func init() {
	// La raíz de datos es el directorio del ejecutable si existe la carpeta
	// "database", si no, el directorio de trabajo actual.
	Raiz, _ = os.Getwd()
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "database")); err == nil {
			Raiz = dir
		}
	}
	CarpetaDBRoot = filepath.Join(Raiz, "database")
	CarpetaBase = filepath.Join(CarpetaDBRoot, "datos_facturacion")
	CarpetaDatos = filepath.Join(CarpetaBase, "datos")
	CarpetaUsers = filepath.Join(CarpetaBase, "usuarios")
	CarpetaChats = filepath.Join(CarpetaBase, "chats")

	for _, d := range []string{CarpetaDBRoot, CarpetaBase, CarpetaDatos, CarpetaUsers, CarpetaChats} {
		_ = os.MkdirAll(d, 0o755)
	}
}

// abrir abre (o crea) una base SQLite con una única conexión serializada,
// equivalente al comportamiento de sqlite3.connect de Python.
func abrir(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite es de un solo escritor: serializamos el acceso.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// filas convierte un resultado SQL en []map[string]any (equivalente a
// [dict(row) for row in cursor.fetchall()] con row_factory = sqlite3.Row).
func filas(rows *sql.Rows) []map[string]any {
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = normalizar(vals[i])
		}
		out = append(out, m)
	}
	return out
}

// fila devuelve una sola fila como mapa (o nil si no hay resultados).
func fila(db *sql.DB, query string, args ...any) map[string]any {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil
	}
	res := filas(rows)
	if len(res) == 0 {
		return nil
	}
	return res[0]
}

// normalizar convierte los tipos crudos de SQLite a tipos JSON-amigables.
// Las fechas se devuelven como "YYYY-MM-DD HH:MM:SS", igual que en Python.
func normalizar(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	default:
		return t
	}
}

// aFloat convierte un valor de SQLite a float64 de forma segura.
func AFloat(v any) float64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return t
	case int64:
		return float64(t)
	case []byte:
		var f float64
		fmt.Sscanf(string(t), "%g", &f)
		return f
	case string:
		var f float64
		fmt.Sscanf(t, "%g", &f)
		return f
	}
	return 0
}

// AInt convierte un valor de SQLite a int64 de forma segura.
func AInt(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case int64:
		return t
	case float64:
		return int64(t)
	case []byte:
		var i int64
		fmt.Sscanf(string(t), "%d", &i)
		return i
	case string:
		var i int64
		fmt.Sscanf(t, "%d", &i)
		return i
	}
	return 0
}

// AStr convierte un valor de SQLite a string de forma segura.
func AStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// StrPtr devuelve nil si la cadena está vacía (para columnas NULL).
func StrPtr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
