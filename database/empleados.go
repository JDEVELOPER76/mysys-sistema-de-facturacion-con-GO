package database

// Empleado extiende User con datos laborales.
type Empleado struct {
	User
	Nombre  string  `json:"nombre"`
	Puesto  string  `json:"puesto"`
	Salario float64 `json:"salario"`
}

// EmpleadoDB gestiona usuarios/empleados (hereda de UserDB).
type EmpleadoDB struct {
	*UserDB
}

// NewEmpleadoDB abre la base de usuarios en modo empleados.
func NewEmpleadoDB(dbName ...string) *EmpleadoDB {
	return &EmpleadoDB{UserDB: NewUserDB(dbName...)}
}

// AgregarEmpleado crea un empleado; devuelve false si el username ya existe.
func (e *EmpleadoDB) AgregarEmpleado(emp Empleado) bool {
	_, err := e.DB.Exec(`
		INSERT INTO users (username, password, tipo, salario, puesto, nombre)
		VALUES (?, ?, ?, ?, ?, ?)`,
		emp.Username, emp.Password, emp.Tipo, emp.Salario, emp.Puesto, emp.Nombre)
	return err == nil
}

// CambiarPassword actualiza la contraseña de un usuario.
func (e *EmpleadoDB) CambiarPassword(username, nuevaClave string) {
	e.DB.Exec(`UPDATE users SET password = ? WHERE username = ?`, nuevaClave, username)
}

// EliminarUsuario borra un usuario por username.
func (e *EmpleadoDB) EliminarUsuario(username string) {
	e.DB.Exec(`DELETE FROM users WHERE username = ?`, username)
}

// ObtenerUsuarios devuelve filas [username, tipo, salario, puesto, nombre]
// en el mismo orden posicional que usaba la versión Python (para las plantillas).
func (e *EmpleadoDB) ObtenerUsuarios() [][]any {
	rows, err := e.DB.Query(`SELECT username, tipo, salario, puesto, nombre FROM users`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := [][]any{}
	for rows.Next() {
		var username, tipo, salario, puesto, nombre string
		if err := rows.Scan(&username, &tipo, &salario, &puesto, &nombre); err != nil {
			continue
		}
		out = append(out, []any{username, tipo, salario, puesto, nombre})
	}
	return out
}
