package database

import (
	"database/sql"
	"path/filepath"
	"strings"
)

// Cliente es el modelo de cliente.
type Cliente struct {
	Nombre             string `json:"nombre"`
	TipoIdentificacion string `json:"tipo_identificacion"`
	Identificacion     string `json:"identificacion"`
	Direccion          string `json:"direccion"`
	Telefono           string `json:"telefono"`
	Email              string `json:"email"`
	FechaRegistro      string `json:"fecha_registro"`
}

// ClienteDB gestiona los clientes en facturacion.db.
type ClienteDB struct {
	DB *sql.DB
}

// NewClienteDB abre facturacion.db y asegura el esquema de clientes.
func NewClienteDB(dbName ...string) *ClienteDB {
	name := "facturacion.db"
	if len(dbName) > 0 && dbName[0] != "" {
		name = dbName[0]
	}
	db, err := abrir(filepath.Join(CarpetaDatos, name))
	if err != nil {
		panic("no se pudo abrir facturacion.db: " + err.Error())
	}
	c := &ClienteDB{DB: db}
	c.crearTabla()
	return c
}

func (c *ClienteDB) crearTabla() {
	c.DB.Exec(`
		CREATE TABLE IF NOT EXISTS clientes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nombre TEXT NOT NULL,
			tipo_identificacion TEXT DEFAULT 'cedula',
			identificacion TEXT UNIQUE,
			direccion TEXT,
			telefono TEXT,
			email TEXT,
			fecha_registro TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)

	// Migraciones: agregar columnas faltantes en bases antiguas.
	rows, err := c.DB.Query(`PRAGMA table_info(clientes)`)
	if err != nil {
		return
	}
	existentes := map[string]bool{}
	for rows.Next() {
		var cid int
		var nombre, tipo string
		var notNull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &nombre, &tipo, &notNull, &dflt, &pk); err == nil {
			existentes[nombre] = true
		}
	}
	rows.Close()

	migraciones := []struct {
		columna string
		ddl     string
	}{
		{"tipo_identificacion", "ALTER TABLE clientes ADD COLUMN tipo_identificacion TEXT DEFAULT 'cedula'"},
		{"identificacion", "ALTER TABLE clientes ADD COLUMN identificacion TEXT UNIQUE"},
		{"direccion", "ALTER TABLE clientes ADD COLUMN direccion TEXT"},
		{"telefono", "ALTER TABLE clientes ADD COLUMN telefono TEXT"},
		{"email", "ALTER TABLE clientes ADD COLUMN email TEXT"},
	}
	for _, m := range migraciones {
		if !existentes[m.columna] {
			_, _ = c.DB.Exec(m.ddl)
		}
	}
}

const columnasClientes = `id, nombre, tipo_identificacion, identificacion, direccion, telefono, email, fecha_registro`

// ObtenerTodosLosClientes devuelve todos los clientes, más reciente primero.
func (c *ClienteDB) ObtenerTodosLosClientes() []map[string]any {
	rows, err := c.DB.Query(`SELECT ` + columnasClientes + ` FROM clientes ORDER BY id DESC`)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ObtenerClientePorID busca un cliente por su ID.
func (c *ClienteDB) ObtenerClientePorID(clienteID int64) map[string]any {
	return fila(c.DB, `SELECT `+columnasClientes+` FROM clientes WHERE id = ?`, clienteID)
}

// ObtenerClientePorIdentificacion busca un cliente por cédula/RUC/pasaporte.
func (c *ClienteDB) ObtenerClientePorIdentificacion(identificacion string) map[string]any {
	return fila(c.DB, `SELECT `+columnasClientes+` FROM clientes WHERE identificacion = ?`, identificacion)
}

// AgregarCliente crea un cliente validando identificación duplicada.
func (c *ClienteDB) AgregarCliente(nombre, tipoIdentificacion string, identificacion, direccion, telefono, email *string) Resultado {
	if tipoIdentificacion == "" {
		tipoIdentificacion = "cedula"
	}
	if identificacion != nil && *identificacion != "" {
		var id int64
		err := c.DB.QueryRow(`SELECT id FROM clientes WHERE identificacion = ?`, *identificacion).Scan(&id)
		if err == nil {
			return Resultado{Success: false, Error: "Ya existe un cliente con esta identificación"}
		}
	}
	res, err := c.DB.Exec(`
		INSERT INTO clientes (nombre, tipo_identificacion, identificacion, direccion, telefono, email)
		VALUES (?, ?, ?, ?, ?, ?)`,
		nombre, tipoIdentificacion, identificacion, direccion, telefono, email)
	if err != nil {
		return Resultado{Success: false, Error: err.Error()}
	}
	id, _ := res.LastInsertId()
	return Resultado{Success: true, ID: id}
}

// ActualizarCliente edita solo los campos proporcionados (no nulos).
func (c *ClienteDB) ActualizarCliente(clienteID int64, nombre, tipoIdentificacion, identificacion, direccion, telefono, email *string) Resultado {
	campos := []string{}
	valores := []any{}
	agregar := func(campo string, valor *string) {
		if valor != nil {
			campos = append(campos, campo+" = ?")
			valores = append(valores, *valor)
		}
	}
	agregar("nombre", nombre)
	agregar("tipo_identificacion", tipoIdentificacion)
	agregar("identificacion", identificacion)
	agregar("direccion", direccion)
	agregar("telefono", telefono)
	agregar("email", email)

	if len(campos) == 0 {
		return Resultado{Success: false, Error: "No se proporcionaron campos para actualizar"}
	}
	valores = append(valores, clienteID)
	_, err := c.DB.Exec(`UPDATE clientes SET `+strings.Join(campos, ", ")+` WHERE id = ?`, valores...)
	if err != nil {
		return Resultado{Success: false, Error: err.Error()}
	}
	return Resultado{Success: true}
}

// EliminarCliente borra un cliente por ID.
func (c *ClienteDB) EliminarCliente(clienteID int64) Resultado {
	_, err := c.DB.Exec(`DELETE FROM clientes WHERE id = ?`, clienteID)
	if err != nil {
		return Resultado{Success: false, Error: err.Error()}
	}
	return Resultado{Success: true}
}

// BuscarClientes busca por nombre o identificación (autocompletado).
func (c *ClienteDB) BuscarClientes(termino string) []map[string]any {
	like := "%" + termino + "%"
	rows, err := c.DB.Query(`
		SELECT `+columnasClientes+`
		FROM clientes
		WHERE nombre LIKE ? OR identificacion LIKE ?
		ORDER BY nombre ASC
		LIMIT 20`, like, like)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ObtenerClientesConGasto devuelve todos los clientes con su total gastado.
func (c *ClienteDB) ObtenerClientesConGasto() []map[string]any {
	rows, err := c.DB.Query(`
		SELECT
			c.id,
			c.nombre,
			c.tipo_identificacion,
			c.identificacion,
			c.direccion,
			c.telefono,
			c.email,
			COALESCE(SUM(v.total), 0) as total_gastado
		FROM clientes c
		LEFT JOIN ventas v ON c.id = v.cliente_id AND v.estado = 'Completada'
		GROUP BY c.id
		ORDER BY c.nombre ASC`)
	if err != nil {
		return nil
	}
	return filas(rows)
}
