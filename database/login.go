package database

import (
	"database/sql"
	"path/filepath"
)

// User es el modelo de usuario (equivalente al modelo Pydantic User).
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Tipo     string `json:"tipo"`
}

// UserDB gestiona la base de datos de usuarios (users.db).
type UserDB struct {
	DB *sql.DB
}

// NewUserDB abre (o crea) la base de usuarios y asegura el esquema.
func NewUserDB(dbName ...string) *UserDB {
	name := "users.db"
	if len(dbName) > 0 && dbName[0] != "" {
		name = dbName[0]
	}
	db, err := abrir(filepath.Join(CarpetaUsers, name))
	if err != nil {
		panic("no se pudo abrir users.db: " + err.Error())
	}
	u := &UserDB{DB: db}
	u.crearTabla()
	return u
}

func (u *UserDB) crearTabla() {
	u.DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			tipo TEXT NOT NULL,
			salario TEXT NOT NULL,
			puesto TEXT NOT NULL,
			nombre TEXT NOT NULL
		)`)
	// Migración: agrega la columna 'foto' si la tabla ya existía sin ella.
	_, _ = u.DB.Exec(`ALTER TABLE users ADD COLUMN foto TEXT`)
}

// ObtenerPerfil devuelve los datos de perfil (apartado 'Perfil' y topbar).
func (u *UserDB) ObtenerPerfil(username string) map[string]any {
	return fila(u.DB,
		`SELECT username, nombre, puesto, tipo, foto FROM users WHERE username = ?`, username)
}

// ObtenerUsuariosBasico devuelve la lista liviana de usuarios (apartado 'En línea').
func (u *UserDB) ObtenerUsuariosBasico() []map[string]any {
	rows, err := u.DB.Query(`SELECT username, nombre, puesto, tipo, foto FROM users`)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ActualizarFoto guarda la ruta de la foto de perfil.
func (u *UserDB) ActualizarFoto(username, fotoURL string) {
	u.DB.Exec(`UPDATE users SET foto = ? WHERE username = ?`, fotoURL, username)
}

// ActualizarDatosBasicos edita nombre y/o puesto del perfil.
func (u *UserDB) ActualizarDatosBasicos(username string, nombre, puesto *string) {
	if nombre != nil {
		u.DB.Exec(`UPDATE users SET nombre = ? WHERE username = ?`, *nombre, username)
	}
	if puesto != nil {
		u.DB.Exec(`UPDATE users SET puesto = ? WHERE username = ?`, *puesto, username)
	}
}

// EsAdmin indica si el usuario tiene rol admin.
func (u *UserDB) EsAdmin(username string) bool {
	var tipo string
	err := u.DB.QueryRow(`SELECT tipo FROM users WHERE username = ?`, username).Scan(&tipo)
	return err == nil && tipo == "admin"
}

// VerificarUsuario comprueba credenciales (texto plano, igual que la versión Python).
func (u *UserDB) VerificarUsuario(username, password string) bool {
	var pass string
	err := u.DB.QueryRow(`SELECT password FROM users WHERE username = ?`, username).Scan(&pass)
	return err == nil && pass == password
}

// ObtenerIDUsuario devuelve el id interno del usuario (0 si no existe).
func (u *UserDB) ObtenerIDUsuario(username string) int64 {
	var id int64
	err := u.DB.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

// HayUsuarios indica si existe al menos un usuario registrado.
func (u *UserDB) HayUsuarios() bool {
	var n int64
	if err := u.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// AgregarUsuario crea un usuario básico (usado por la app de registro).
func (u *UserDB) AgregarUsuario(user User) {
	u.DB.Exec(`INSERT INTO users (username, password, tipo, salario, puesto, nombre) VALUES (?, ?, ?, ?, ?, ?)`,
		user.Username, user.Password, user.Tipo, "0", "nulo", "nulo")
}
