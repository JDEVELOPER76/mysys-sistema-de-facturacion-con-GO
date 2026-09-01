package database

import (
	"database/sql"
	"path/filepath"
)

// Producto es el modelo de producto del inventario.
type Producto struct {
	Nombre       string  `json:"nombre"`
	Descripcion  string  `json:"descripcion"`
	Precio       float64 `json:"precio"`
	IVA          float64 `json:"iva"`
	CodigoBarras string  `json:"codigo_barras"`
	Proveedor    string  `json:"proveedor"`
	Stock        int64   `json:"stock"`
	Categoria    string  `json:"categoria"`
	ImagenURL    string  `json:"imagen_url"`
}

// Resultado es la respuesta estándar de las operaciones de escritura.
type Resultado struct {
	Success bool   `json:"success"`
	ID      int64  `json:"id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ProductoDB gestiona el inventario en facturacion.db.
type ProductoDB struct {
	DB *sql.DB
}

// NewProductoDB abre facturacion.db y asegura la tabla productos.
func NewProductoDB(dbName ...string) *ProductoDB {
	name := "facturacion.db"
	if len(dbName) > 0 && dbName[0] != "" {
		name = dbName[0]
	}
	db, err := abrir(filepath.Join(CarpetaDatos, name))
	if err != nil {
		panic("no se pudo abrir facturacion.db: " + err.Error())
	}
	p := &ProductoDB{DB: db}
	p.crearTabla()
	return p
}

func (p *ProductoDB) crearTabla() {
	p.DB.Exec(`
		CREATE TABLE IF NOT EXISTS productos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nombre TEXT NOT NULL,
			descripcion TEXT,
			precio REAL NOT NULL,
			iva REAL DEFAULT 12.0,
			codigo_barras TEXT UNIQUE NOT NULL,
			proveedor TEXT NOT NULL,
			stock INTEGER DEFAULT 0,
			categoria TEXT NOT NULL,
			imagen_url TEXT,
			activo INTEGER DEFAULT 1,
			fecha_creacion TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			fecha_actualizacion TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
}

// AgregarProducto inserta un producto; falla si el código de barras ya existe.
func (p *ProductoDB) AgregarProducto(prod Producto) Resultado {
	res, err := p.DB.Exec(`
		INSERT INTO productos (nombre, descripcion, precio, iva, codigo_barras, proveedor, stock, categoria, imagen_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		prod.Nombre, StrPtr(prod.Descripcion), prod.Precio, prod.IVA,
		prod.CodigoBarras, prod.Proveedor, prod.Stock, prod.Categoria, StrPtr(prod.ImagenURL))
	if err != nil {
		return Resultado{Success: false, Error: "El código de barras ya existe: " + err.Error()}
	}
	id, _ := res.LastInsertId()
	return Resultado{Success: true, ID: id}
}

// ObtenerProducto busca un producto activo por ID.
func (p *ProductoDB) ObtenerProducto(productoID int64) map[string]any {
	return fila(p.DB, `SELECT * FROM productos WHERE id = ? AND activo = 1`, productoID)
}

// ObtenerProductoPorCodigo busca un producto activo por código de barras.
func (p *ProductoDB) ObtenerProductoPorCodigo(codigoBarras string) map[string]any {
	return fila(p.DB, `SELECT * FROM productos WHERE codigo_barras = ? AND activo = 1`, codigoBarras)
}

// ListarProductos devuelve el inventario activo, más reciente primero.
func (p *ProductoDB) ListarProductos(limite, offset int64) []map[string]any {
	if limite <= 0 {
		limite = 100
	}
	rows, err := p.DB.Query(`
		SELECT * FROM productos
		WHERE activo = 1
		ORDER BY fecha_creacion DESC
		LIMIT ? OFFSET ?`, limite, offset)
	if err != nil {
		return nil
	}
	return filas(rows)
}

// ActualizarProducto edita un producto existente.
func (p *ProductoDB) ActualizarProducto(productoID int64, prod Producto) Resultado {
	_, err := p.DB.Exec(`
		UPDATE productos
		SET nombre = ?, descripcion = ?, precio = ?, iva = ?, codigo_barras = ?,
			proveedor = ?, stock = ?, categoria = ?, imagen_url = ?,
			fecha_actualizacion = CURRENT_TIMESTAMP
		WHERE id = ?`,
		prod.Nombre, StrPtr(prod.Descripcion), prod.Precio, prod.IVA,
		prod.CodigoBarras, prod.Proveedor, prod.Stock, prod.Categoria,
		StrPtr(prod.ImagenURL), productoID)
	if err != nil {
		return Resultado{Success: false, Error: err.Error()}
	}
	return Resultado{Success: true}
}

// EliminarProducto hace borrado lógico (activo = 0).
func (p *ProductoDB) EliminarProducto(productoID int64) Resultado {
	_, err := p.DB.Exec(`UPDATE productos SET activo = 0 WHERE id = ?`, productoID)
	if err != nil {
		return Resultado{Success: false, Error: err.Error()}
	}
	return Resultado{Success: true}
}
