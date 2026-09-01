package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/shopspring/decimal"
)

// RedondearMonto redondea a 2 decimales con ROUND_HALF_UP (igual que Python Decimal).
// NewFromFloat usa la representación decimal más corta (como Decimal(str(v))),
// y Round(2) redondea half-up: reproduce exactamente Decimal.quantize(0.01, HALF_UP).
func RedondearMonto(valor float64) float64 {
	f, _ := decimal.NewFromFloat(valor).Round(2).Float64()
	return f
}

// DetalleVenta es una línea del carrito de compras.
type DetalleVenta struct {
	ProductoID     int64   `json:"producto_id"`
	Cantidad       int64   `json:"cantidad"`
	PrecioUnitario float64 `json:"precio_unitario"`
	IVA            float64 `json:"iva"`
}

// Venta es el modelo de una venta del punto de venta.
type Venta struct {
	ClienteID  *int64         `json:"cliente_id"`
	MetodoPago string         `json:"metodo_pago"`
	Detalles   []DetalleVenta `json:"detalles"`
	EmpleadoID *int64         `json:"empleado_id"`
	Total      *float64       `json:"total"`
}

// VentaDB gestiona ventas y detalles en facturacion.db, con users.db
// adjunta como db_usuarios (igual que la versión Python con ATTACH).
type VentaDB struct {
	DB *sql.DB
}

// NewVentaDB abre facturacion.db, adjunta users.db y asegura el esquema.
func NewVentaDB(dbName ...string) *VentaDB {
	name := "facturacion.db"
	if len(dbName) > 0 && dbName[0] != "" {
		name = dbName[0]
	}
	db, err := abrir(filepath.Join(CarpetaDatos, name))
	if err != nil {
		panic("no se pudo abrir facturacion.db: " + err.Error())
	}
	v := &VentaDB{DB: db}
	v.crearTablas()
	return v
}

// adjuntarUsuarios adjunta users.db como db_usuarios en la conexión actual.
// Debe llamarse antes de cualquier consulta que use db_usuarios.users.
func (v *VentaDB) adjuntarUsuarios(conn *sql.Conn) {
	usersPath := filepath.Join(CarpetaUsers, "users.db")
	_, _ = conn.ExecContext(ctx(), fmt.Sprintf(`ATTACH DATABASE '%s' AS db_usuarios`, usersPath))
}

// consulta ejecuta una query asegurando que users.db está adjunta.
func (v *VentaDB) consulta(query string, args ...any) []map[string]any {
	conn, err := v.DB.Conn(ctx())
	if err != nil {
		return nil
	}
	defer conn.Close()
	v.adjuntarUsuarios(conn)
	rows, err := conn.QueryContext(ctx(), query, args...)
	if err != nil {
		return nil
	}
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

func (v *VentaDB) crearTablas() {
	v.DB.Exec(`
		CREATE TABLE IF NOT EXISTS ventas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cliente_id INTEGER,
			empleado_id INTEGER,
			total NUMERIC(10,2) NOT NULL,
			metodo_pago TEXT DEFAULT 'Efectivo',
			estado TEXT DEFAULT 'Completada',
			fecha_venta TEXT,
			fecha_completa TEXT,
			FOREIGN KEY(cliente_id) REFERENCES clientes(id)
		)`)
	v.DB.Exec(`
		CREATE TABLE IF NOT EXISTS detalles_ventas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			venta_id INTEGER,
			producto_id INTEGER,
			cantidad INTEGER,
			precio_unitario NUMERIC(10,2),
			iva NUMERIC(5,2),
			subtotal NUMERIC(10,2),
			FOREIGN KEY(venta_id) REFERENCES ventas(id)
		)`)
}

// RegistrarVenta inserta la venta, sus detalles y descuenta el stock (transacción).
func (v *VentaDB) RegistrarVenta(venta Venta, fDia, fCompleta string) map[string]any {
	if fDia == "" || fCompleta == "" {
		ahora := time.Now()
		fDia = ahora.Format("2006-01-02")
		fCompleta = ahora.Format("2006-01-02 15:04:05")
	}

	tx, err := v.DB.Begin()
	if err != nil {
		return map[string]any{"success": false, "error": err.Error()}
	}
	defer tx.Rollback()

	total := 0.0
	if venta.Total != nil {
		total = *venta.Total
	}
	metodo := venta.MetodoPago
	if metodo == "" {
		metodo = "Efectivo"
	}

	res, err := tx.Exec(`
		INSERT INTO ventas (cliente_id, empleado_id, total, metodo_pago, fecha_venta, fecha_completa)
		VALUES (?, ?, ?, ?, ?, ?)`,
		venta.ClienteID, venta.EmpleadoID, RedondearMonto(total), metodo, fDia, fCompleta)
	if err != nil {
		return map[string]any{"success": false, "error": err.Error()}
	}
	ventaID, _ := res.LastInsertId()

	for _, d := range venta.Detalles {
		precioUnitario := RedondearMonto(d.PrecioUnitario)
		iva := RedondearMonto(d.IVA)
		precioConIVA := RedondearMonto(precioUnitario * (1 + iva/100))
		subtotal := RedondearMonto(float64(d.Cantidad) * precioConIVA)

		if _, err := tx.Exec(`
			INSERT INTO detalles_ventas (venta_id, producto_id, cantidad, precio_unitario, iva, subtotal)
			VALUES (?, ?, ?, ?, ?, ?)`,
			ventaID, d.ProductoID, d.Cantidad, precioUnitario, iva, subtotal); err != nil {
			return map[string]any{"success": false, "error": err.Error()}
		}
		if _, err := tx.Exec(`UPDATE productos SET stock = stock - ? WHERE id = ?`,
			d.Cantidad, d.ProductoID); err != nil {
			return map[string]any{"success": false, "error": err.Error()}
		}
	}

	if err := tx.Commit(); err != nil {
		return map[string]any{"success": false, "error": err.Error()}
	}
	return map[string]any{"success": true, "venta_id": ventaID}
}

// productosDeVenta devuelve las líneas de detalle de una venta.
func (v *VentaDB) productosDeVenta(conn *sql.Conn, ventaID int64) []map[string]any {
	rows, err := conn.QueryContext(ctx(), `
		SELECT dv.id, dv.cantidad, dv.precio_unitario, dv.iva, dv.subtotal, p.nombre AS nombre_producto
		FROM detalles_ventas dv
		JOIN productos p ON dv.producto_id = p.id
		WHERE dv.venta_id = ?`, ventaID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	cols, _ := rows.Columns()
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

// ListarUltimasVentas devuelve las últimas ventas con cliente, empleado y productos.
func (v *VentaDB) ListarUltimasVentas(limite int64) []map[string]any {
	if limite <= 0 {
		limite = 20
	}
	conn, err := v.DB.Conn(ctx())
	if err != nil {
		return nil
	}
	defer conn.Close()
	v.adjuntarUsuarios(conn)

	rows, err := conn.QueryContext(ctx(), `
		SELECT v.id, v.total, v.metodo_pago, v.estado, v.fecha_completa, v.fecha_venta,
		       c.nombre AS nombre_cliente,
		       u.username AS nombre_empleado
		FROM ventas v
		LEFT JOIN clientes c ON v.cliente_id = c.id
		LEFT JOIN db_usuarios.users u ON v.empleado_id = u.id
		ORDER BY v.id DESC
		LIMIT ?`, limite)
	if err != nil {
		return nil
	}
	cols, _ := rows.Columns()
	ventas := []map[string]any{}
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
		ventas = append(ventas, m)
	}
	rows.Close()

	for _, venta := range ventas {
		venta["productos"] = v.productosDeVenta(conn, AInt(venta["id"]))
	}
	return ventas
}

// ObtenerVentasTotalesPorDia agrupa ventas completadas por día.
func (v *VentaDB) ObtenerVentasTotalesPorDia() []map[string]any {
	return v.consulta(`
		SELECT fecha_venta AS dia,
		       COUNT(id) AS cantidad_ventas,
		       SUM(total) AS total_recaudado
		FROM ventas
		WHERE estado = 'Completada'
		GROUP BY fecha_venta
		ORDER BY fecha_venta DESC`)
}

// ObtenerTotalVentas devuelve el total histórico de ventas completadas.
func (v *VentaDB) ObtenerTotalVentas() float64 {
	row := fila(v.DB, `SELECT SUM(total) AS total_historico FROM ventas WHERE estado = 'Completada'`)
	if row == nil {
		return 0
	}
	return AFloat(row["total_historico"])
}

// ObtenerTotalVentasHoy devuelve el total vendido en el día actual.
func (v *VentaDB) ObtenerTotalVentasHoy() float64 {
	hoy := time.Now().Format("2006-01-02")
	row := fila(v.DB, `SELECT SUM(total) AS total_hoy FROM ventas WHERE estado = 'Completada' AND fecha_venta = ?`, hoy)
	if row == nil {
		return 0
	}
	return AFloat(row["total_hoy"])
}

// ObtenerVentaConProductos devuelve una venta con su desglose de productos.
func (v *VentaDB) ObtenerVentaConProductos(ventaID int64) map[string]any {
	conn, err := v.DB.Conn(ctx())
	if err != nil {
		return nil
	}
	defer conn.Close()
	v.adjuntarUsuarios(conn)

	rows, err := conn.QueryContext(ctx(), `
		SELECT v.id, v.total, v.metodo_pago, v.estado, v.fecha_completa, v.fecha_venta,
		       c.nombre AS nombre_cliente,
		       u.username AS nombre_empleado
		FROM ventas v
		LEFT JOIN clientes c ON v.cliente_id = c.id
		LEFT JOIN db_usuarios.users u ON v.empleado_id = u.id
		WHERE v.id = ?`, ventaID)
	if err != nil {
		return nil
	}
	cols, _ := rows.Columns()
	var venta map[string]any
	if rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err == nil {
			venta = map[string]any{}
			for i, c := range cols {
				venta[c] = normalizar(vals[i])
			}
		}
	}
	rows.Close()
	if venta == nil {
		return map[string]any{}
	}
	venta["productos"] = v.productosDeVenta(conn, ventaID)
	return venta
}

// ObtenerMetodosPago agrupa ventas por método de pago.
func (v *VentaDB) ObtenerMetodosPago() []map[string]any {
	return v.consulta(`
		SELECT metodo_pago, COUNT(id) AS cantidad, SUM(total) AS total
		FROM ventas
		WHERE estado = 'Completada'
		GROUP BY metodo_pago
		ORDER BY cantidad DESC`)
}

// ObtenerTopProductos devuelve los productos más vendidos.
func (v *VentaDB) ObtenerTopProductos(limite int64) []map[string]any {
	return v.consulta(`
		SELECT p.nombre AS nombre_producto,
			SUM(dv.cantidad) AS cantidad_vendida,
			SUM(dv.subtotal) AS total_generado
		FROM detalles_ventas dv
		JOIN productos p ON dv.producto_id = p.id
		JOIN ventas v ON dv.venta_id = v.id
		WHERE v.estado = 'Completada'
		GROUP BY dv.producto_id
		ORDER BY cantidad_vendida DESC
		LIMIT ?`, limite)
}

// ObtenerTopClientes devuelve los clientes que más han gastado.
func (v *VentaDB) ObtenerTopClientes(limite int64) []map[string]any {
	return v.consulta(`
		SELECT c.nombre AS nombre_cliente,
			COUNT(v.id) AS cantidad_compras,
			SUM(v.total) AS total_gastado
		FROM ventas v
		JOIN clientes c ON v.cliente_id = c.id
		WHERE v.estado = 'Completada'
		GROUP BY v.cliente_id
		ORDER BY total_gastado DESC
		LIMIT ?`, limite)
}

// ObtenerRendimientoEmpleados devuelve el desempeño de cada empleado.
func (v *VentaDB) ObtenerRendimientoEmpleados() []map[string]any {
	resultados := v.consulta(`
		SELECT u.username AS nombre_empleado,
			COUNT(v.id) AS cantidad_ventas,
			SUM(v.total) AS total_facturado,
			AVG(v.total) AS ticket_promedio
		FROM ventas v
		JOIN db_usuarios.users u ON v.empleado_id = u.id
		WHERE v.estado = 'Completada'
		GROUP BY v.empleado_id
		ORDER BY total_facturado DESC`)

	if len(resultados) > 0 {
		maxTotal := AFloat(resultados[0]["total_facturado"])
		for _, r := range resultados {
			if maxTotal > 0 {
				r["porcentaje"] = AFloat(r["total_facturado"]) / maxTotal * 100
			} else {
				r["porcentaje"] = 0.0
			}
		}
	}
	return resultados
}

// ==========================================
//   MÉTODOS POR PERIODO (reportes)
// ==========================================

func fmtFecha(t time.Time) string { return t.Format("2006-01-02") }

// ObtenerVentasPorPeriodo lista ventas completadas entre dos fechas.
func (v *VentaDB) ObtenerVentasPorPeriodo(desde, hasta time.Time) []map[string]any {
	return v.consulta(`
		SELECT
			v.id,
			v.total,
			v.metodo_pago,
			v.fecha_completa,
			c.nombre as nombre_cliente,
			u.username as nombre_empleado
		FROM ventas v
		LEFT JOIN clientes c ON v.cliente_id = c.id
		LEFT JOIN db_usuarios.users u ON v.empleado_id = u.id
		WHERE DATE(COALESCE(v.fecha_completa, v.fecha_venta)) BETWEEN DATE(?) AND DATE(?)
		AND v.estado = 'Completada'
		ORDER BY v.fecha_completa DESC`, fmtFecha(desde), fmtFecha(hasta))
}

// ObtenerVentasTotalesPorDiaPeriodo agrupa ventas por día dentro del rango.
func (v *VentaDB) ObtenerVentasTotalesPorDiaPeriodo(desde, hasta time.Time) []map[string]any {
	return v.consulta(`
		SELECT
			DATE(fecha_completa) as dia,
			COUNT(*) as cantidad_ventas,
			SUM(total) as total_recaudado
		FROM ventas
		WHERE DATE(COALESCE(fecha_completa, fecha_venta)) BETWEEN DATE(?) AND DATE(?)
		AND estado = 'Completada'
		GROUP BY DATE(fecha_completa)
		ORDER BY dia DESC`, fmtFecha(desde), fmtFecha(hasta))
}

// ObtenerMetodosPagoPeriodo agrupa por método de pago dentro del rango.
func (v *VentaDB) ObtenerMetodosPagoPeriodo(desde, hasta time.Time) []map[string]any {
	return v.consulta(`
		SELECT
			metodo_pago,
			COUNT(*) as cantidad,
			SUM(total) as total_recaudado
		FROM ventas
		WHERE DATE(COALESCE(fecha_completa, fecha_venta)) BETWEEN DATE(?) AND DATE(?)
		AND estado = 'Completada'
		GROUP BY metodo_pago
		ORDER BY cantidad DESC`, fmtFecha(desde), fmtFecha(hasta))
}

// ObtenerRendimientoEmpleadosPeriodo devuelve desempeño por empleado en el rango.
func (v *VentaDB) ObtenerRendimientoEmpleadosPeriodo(desde, hasta time.Time) []map[string]any {
	return v.consulta(`
		SELECT
			u.username as nombre_empleado,
			COUNT(v.id) as cantidad_ventas,
			SUM(v.total) as total_facturado,
			AVG(v.total) as ticket_promedio
		FROM ventas v
		JOIN db_usuarios.users u ON v.empleado_id = u.id
		WHERE DATE(COALESCE(v.fecha_completa, v.fecha_venta)) BETWEEN DATE(?) AND DATE(?)
		AND v.estado = 'Completada'
		GROUP BY u.id
		ORDER BY total_facturado DESC`, fmtFecha(desde), fmtFecha(hasta))
}

// ObtenerTopProductosPeriodo devuelve los más vendidos en el rango.
func (v *VentaDB) ObtenerTopProductosPeriodo(desde, hasta time.Time, limite int64) []map[string]any {
	return v.consulta(`
		SELECT p.nombre AS nombre_producto, SUM(dv.cantidad) AS cantidad_vendida,
		       SUM(dv.subtotal) AS total_generado
		FROM detalles_ventas dv
		JOIN productos p ON dv.producto_id = p.id
		JOIN ventas v ON dv.venta_id = v.id
		WHERE v.estado = 'Completada' AND DATE(COALESCE(v.fecha_completa, v.fecha_venta)) BETWEEN DATE(?) AND DATE(?)
		GROUP BY dv.producto_id ORDER BY cantidad_vendida DESC LIMIT ?`,
		fmtFecha(desde), fmtFecha(hasta), limite)
}

// ObtenerTopClientesPeriodo devuelve los mejores clientes en el rango.
func (v *VentaDB) ObtenerTopClientesPeriodo(desde, hasta time.Time, limite int64) []map[string]any {
	return v.consulta(`
		SELECT c.nombre AS nombre_cliente, COUNT(v.id) AS cantidad_compras,
		       SUM(v.total) AS total_gastado
		FROM ventas v JOIN clientes c ON v.cliente_id = c.id
		WHERE v.estado = 'Completada' AND DATE(COALESCE(v.fecha_completa, v.fecha_venta)) BETWEEN DATE(?) AND DATE(?)
		GROUP BY v.cliente_id ORDER BY total_gastado DESC LIMIT ?`,
		fmtFecha(desde), fmtFecha(hasta), limite)
}

// ObtenerProductosVendidosEntre cuenta unidades vendidas en el rango.
func (v *VentaDB) ObtenerProductosVendidosEntre(desde, hasta time.Time) int64 {
	row := fila(v.DB, `
		SELECT COALESCE(SUM(dv.cantidad), 0) AS total
		FROM detalles_ventas dv
		JOIN ventas v ON dv.venta_id = v.id
		WHERE v.estado = 'Completada'
		  AND DATE(COALESCE(v.fecha_completa, v.fecha_venta)) BETWEEN DATE(?) AND DATE(?)`,
		fmtFecha(desde), fmtFecha(hasta))
	if row == nil {
		return 0
	}
	return AInt(row["total"])
}
