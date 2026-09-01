package main

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mysys/database"
)

// parsearRango interpreta desde/hasta (YYYY-MM-DD): hasta se extiende a 23:59:59.
func parsearRango(desde, hasta string) (time.Time, time.Time, bool) {
	ini, err1 := time.ParseInLocation("2006-01-02", desde, time.Local)
	fin, err2 := time.ParseInLocation("2006-01-02", hasta, time.Local)
	if err1 != nil || err2 != nil {
		return ini, fin, false
	}
	fin = fin.Add(24*time.Hour - time.Second)
	return ini, fin, true
}

// ==========================================
//   ESTADÍSTICAS
// ==========================================

func vistaEstadisticas(w http.ResponseWriter, r *http.Request) {
	username := requiereAdmin(w, r)
	if username == "" {
		return
	}

	// 1. Leer y validar el filtro de la query string
	q := r.URL.Query()
	desdeRaw := strings.TrimSpace(q.Get("desde"))
	hastaRaw := strings.TrimSpace(q.Get("hasta"))
	periodoRaw := strings.TrimSpace(q.Get("periodo"))
	if periodoRaw == "" {
		periodoRaw = "30"
	}
	var errorFiltro string

	periodosValidos := map[string]bool{"7": true, "15": true, "30": true, "90": true, "365": true}
	dias := 30
	if periodosValidos[periodoRaw] {
		dias, _ = strconv.Atoi(periodoRaw)
	} else if _, err := strconv.Atoi(periodoRaw); err != nil {
		errorFiltro = "Periodo inválido, usando 30 días"
	}

	hoy := time.Now()
	inicio := hoy.AddDate(0, 0, -(dias - 1))
	fin := time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 23, 59, 59, 0, time.Local)

	if desdeRaw != "" {
		if t, err := time.ParseInLocation("2006-01-02", desdeRaw, time.Local); err == nil {
			inicio = t
		} else {
			errorFiltro = "Fecha 'desde' inválida, usando periodo seleccionado"
		}
	}
	if hastaRaw != "" {
		if t, err := time.ParseInLocation("2006-01-02", hastaRaw, time.Local); err == nil {
			fin = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
		} else {
			errorFiltro = "Fecha 'hasta' inválida, usando periodo seleccionado"
		}
	}
	if inicio.After(fin) {
		inicio, fin = fin, inicio
		if errorFiltro == "" {
			errorFiltro = "El rango se invirtió automáticamente"
		}
	}

	// 2. Consultar la BD filtrada por el rango
	ventas := ventasDB.ObtenerVentasPorPeriodo(inicio, fin)
	ventasPorDia := ventasDB.ObtenerVentasTotalesPorDiaPeriodo(inicio, fin)
	metodosPago := ventasDB.ObtenerMetodosPagoPeriodo(inicio, fin)
	topProductos := ventasDB.ObtenerTopProductosPeriodo(inicio, fin, 5)
	topClientes := ventasDB.ObtenerTopClientesPeriodo(inicio, fin, 5)
	empleadosVentas := ventasDB.ObtenerRendimientoEmpleadosPeriodo(inicio, fin)

	// 3. Calcular métricas
	totalPeriodo, ticketPromedio, productosVendidos := metricasPeriodo(ventas, inicio, fin)
	aplicarPorcentaje(empleadosVentas)

	render(w, r, "admin_estadistica.html", map[string]any{
		"username":         username,
		"foto":             obtenerFotoUsuario(username),
		"ventas":           ventas,
		"ventas_por_dia":   ventasPorDia,
		"metodos_pago":     metodosPago,
		"top_productos":    topProductos,
		"top_clientes":     topClientes,
		"empleados_ventas": empleadosVentas,
		"stats_payload": map[string]any{
			"ventas_por_dia": ventasPorDia,
			"metodos_pago":   metodosPago,
			"top_productos":  topProductos,
			"top_clientes":   topClientes,
		},
		"total_periodo":        redondearMonto(totalPeriodo),
		"total_ventas_periodo": len(ventas),
		"ticket_promedio":      redondearMonto(ticketPromedio),
		"productos_vendidos":   productosVendidos,
		"filtro": map[string]any{
			"desde":   inicio.Format("2006-01-02"),
			"hasta":   fin.Format("2006-01-02"),
			"periodo": dias,
		},
		"error_filtro": nilOrString(errorFiltro),
	})
}

func nilOrString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// metricasPeriodo calcula total, ticket promedio y unidades vendidas.
func metricasPeriodo(ventas []map[string]any, inicio, fin time.Time) (float64, float64, int64) {
	total := 0.0
	for _, v := range ventas {
		total += database.AFloat(v["total"])
	}
	productosVendidos := ventasDB.ObtenerProductosVendidosEntre(inicio, fin)
	ticket := 0.0
	if len(ventas) > 0 {
		ticket = total / float64(len(ventas))
	}
	return total, ticket, productosVendidos
}

// aplicarPorcentaje añade el campo 'porcentaje' relativo al mejor empleado.
func aplicarPorcentaje(empleados []map[string]any) {
	maxTotal := 0.0
	for _, e := range empleados {
		if t := database.AFloat(e["total_facturado"]); t > maxTotal {
			maxTotal = t
		}
	}
	for _, e := range empleados {
		if maxTotal > 0 {
			e["porcentaje"] = database.AFloat(e["total_facturado"]) / maxTotal * 100
		} else {
			e["porcentaje"] = 0.0
		}
	}
}

func apiEstadisticas(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	desde := r.URL.Query().Get("desde")
	hasta := r.URL.Query().Get("hasta")
	inicio, fin, ok := parsearRango(desde, hasta)
	if !ok {
		jsonError(w, http.StatusBadRequest, "Las fechas deben tener formato YYYY-MM-DD")
		return
	}
	if inicio.After(fin) {
		jsonError(w, http.StatusBadRequest, "La fecha Desde no puede ser posterior a Hasta")
		return
	}

	ventas := ventasDB.ObtenerVentasPorPeriodo(inicio, fin)
	ventasPorDia := ventasDB.ObtenerVentasTotalesPorDiaPeriodo(inicio, fin)
	metodosPago := ventasDB.ObtenerMetodosPagoPeriodo(inicio, fin)
	topProductos := ventasDB.ObtenerTopProductosPeriodo(inicio, fin, 5)
	topClientes := ventasDB.ObtenerTopClientesPeriodo(inicio, fin, 5)
	empleados := ventasDB.ObtenerRendimientoEmpleadosPeriodo(inicio, fin)

	total, ticket, productos := metricasPeriodo(ventas, inicio, fin)
	aplicarPorcentaje(empleados)

	jsonResponde(w, http.StatusOK, map[string]any{
		"desde": desde, "hasta": hasta, "ventas_por_dia": ventasPorDia,
		"metodos_pago": metodosPago, "top_productos": topProductos,
		"top_clientes": topClientes, "empleados_ventas": empleados,
		"total_periodo": total, "total_ventas_periodo": len(ventas),
		"ticket_promedio": ticket, "productos_vendidos": productos,
	})
}

// ==========================================
//   REPORTES (API JSON)
// ==========================================

func reporteVentas(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	desde := r.URL.Query().Get("desde")
	hasta := r.URL.Query().Get("hasta")
	ini, fin, ok := parsearRango(desde, hasta)
	if !ok {
		jsonError(w, http.StatusBadRequest, "Formato de fecha inválido. Usa YYYY-MM-DD.")
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{
		"ventas":         ventasDB.ObtenerVentasPorPeriodo(ini, fin),
		"ventas_por_dia": ventasDB.ObtenerVentasTotalesPorDiaPeriodo(ini, fin),
		"metodos_pago":   ventasDB.ObtenerMetodosPagoPeriodo(ini, fin),
	})
}

func reporteAuditoria(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	ini, fin, ok := parsearRango(r.URL.Query().Get("desde"), r.URL.Query().Get("hasta"))
	if !ok {
		jsonError(w, http.StatusBadRequest, "Formato de fecha inválido. Usa YYYY-MM-DD.")
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{"logs": auditoriaDB.ObtenerLogsPorPeriodo(ini, fin)})
}

func reporteProductos(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{"productos": dbProductos.ListarProductos(100, 0)})
}

func reporteClientes(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{"clientes": clienteDB.ObtenerClientesConGasto()})
}

func reporteEmpleados(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	ini, fin, ok := parsearRango(r.URL.Query().Get("desde"), r.URL.Query().Get("hasta"))
	if !ok {
		jsonError(w, http.StatusBadRequest, "Formato de fecha inválido. Usa YYYY-MM-DD.")
		return
	}
	jsonResponde(w, http.StatusOK, map[string]any{"empleados": ventasDB.ObtenerRendimientoEmpleadosPeriodo(ini, fin)})
}

// ==========================================
//   EXPORTACIÓN A EXCEL
// ==========================================

// generarReporteExcel crea el .xlsx y lo envía como descarga.
func generarReporteExcel(w http.ResponseWriter, data []map[string]any, columnas []Columna, nombre string) {
	buf, err := crearExcelDesdeDatos(data, columnas)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error al generar el Excel: "+err.Error())
		return
	}
	fechaActual := time.Now().Format("20060102_150405")
	nombreArchivo := nombre + "_" + fechaActual + ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename="+nombreArchivo)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

func exportarVentasExcel(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	desde := r.URL.Query().Get("desde")
	hasta := r.URL.Query().Get("hasta")
	ini, fin, ok := parsearRango(desde, hasta)
	if !ok {
		jsonError(w, http.StatusBadRequest, "Formato de fecha inválido.")
		return
	}
	ventas := ventasDB.ObtenerVentasPorPeriodo(ini, fin)
	columnas := []Columna{
		{"id", "ID Venta"},
		{"nombre_cliente", "Cliente"},
		{"nombre_empleado", "Empleado"},
		{"fecha_completa", "Fecha"},
		{"metodo_pago", "Método de Pago"},
		{"total", "Total ($)"},
	}
	generarReporteExcel(w, ventas, columnas, "reporte_ventas")
}

func exportarAuditoriaExcel(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	ini, fin, ok := parsearRango(r.URL.Query().Get("desde"), r.URL.Query().Get("hasta"))
	if !ok {
		jsonError(w, http.StatusBadRequest, "Formato de fecha inválido.")
		return
	}
	logs := auditoriaDB.ObtenerLogsPorPeriodo(ini, fin)
	columnas := []Columna{
		{"fecha_hora", "Fecha/Hora"},
		{"usuario", "Usuario"},
		{"accion", "Acción"},
		{"tabla", "Tabla"},
		{"detalles", "Detalles"},
	}
	generarReporteExcel(w, logs, columnas, "reporte_auditoria")
}

func exportarProductosExcel(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	productos := dbProductos.ListarProductos(100, 0)
	columnas := []Columna{
		{"id", "ID"},
		{"nombre", "Nombre"},
		{"codigo_barras", "Código de Barras"},
		{"proveedor", "Proveedor"},
		{"categoria", "Categoría"},
		{"precio", "Precio ($)"},
		{"stock", "Stock"},
		{"iva", "IVA (%)"},
	}
	generarReporteExcel(w, productos, columnas, "reporte_productos")
}

func exportarClientesExcel(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	clientes := clienteDB.ObtenerClientesConGasto()
	columnas := []Columna{
		{"id", "ID"},
		{"nombre", "Nombre"},
		{"identificacion", "Identificación"},
		{"telefono", "Teléfono"},
		{"email", "Email"},
		{"total_gastado", "Total Gastado ($)"},
	}
	generarReporteExcel(w, clientes, columnas, "reporte_clientes")
}

func exportarEmpleadosExcel(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	ini, fin, ok := parsearRango(r.URL.Query().Get("desde"), r.URL.Query().Get("hasta"))
	if !ok {
		jsonError(w, http.StatusBadRequest, "Formato de fecha inválido.")
		return
	}
	empleados := ventasDB.ObtenerRendimientoEmpleadosPeriodo(ini, fin)
	columnas := []Columna{
		{"nombre_empleado", "Empleado"},
		{"cantidad_ventas", "Ventas"},
		{"total_facturado", "Total Facturado ($)"},
		{"ticket_promedio", "Ticket Promedio ($)"},
	}
	generarReporteExcel(w, empleados, columnas, "reporte_empleados")
}

// ==========================================
//   EXPORTACIÓN A PDF
// ==========================================

func exportarReportePDF(w http.ResponseWriter, r *http.Request) {
	if requiereAdminAPI(w, r) == "" {
		return
	}
	desde := r.URL.Query().Get("desde")
	hasta := r.URL.Query().Get("hasta")
	tipo := r.URL.Query().Get("tipo")
	ini, fin, ok := parsearRango(desde, hasta)
	if !ok {
		jsonError(w, http.StatusBadRequest, "Formato de fecha inválido.")
		return
	}

	var data []map[string]any
	var columnas []Columna
	var titulo string

	switch tipo {
	case "ventas":
		data = ventasDB.ObtenerVentasPorPeriodo(ini, fin)
		columnas = []Columna{
			{"id", "ID"},
			{"nombre_cliente", "Cliente"},
			{"nombre_empleado", "Empleado"},
			{"fecha_completa", "Fecha"},
			{"metodo_pago", "Método Pago"},
			{"total", "Total ($)"},
		}
		titulo = "Reporte de Ventas"
	case "auditoria":
		data = auditoriaDB.ObtenerLogsPorPeriodo(ini, fin)
		columnas = []Columna{
			{"fecha_hora", "Fecha/Hora"},
			{"usuario", "Usuario"},
			{"accion", "Acción"},
			{"tabla", "Tabla"},
			{"detalles", "Detalles"},
		}
		titulo = "Reporte de Auditoría"
	case "productos":
		data = dbProductos.ListarProductos(100, 0)
		columnas = []Columna{
			{"nombre", "Nombre"},
			{"codigo_barras", "Código"},
			{"proveedor", "Proveedor"},
			{"categoria", "Categoría"},
			{"precio", "Precio ($)"},
			{"stock", "Stock"},
			{"iva", "IVA (%)"},
		}
		titulo = "Reporte de Productos"
	case "clientes":
		data = clienteDB.ObtenerClientesConGasto()
		columnas = []Columna{
			{"nombre", "Nombre"},
			{"identificacion", "Identificación"},
			{"telefono", "Teléfono"},
			{"email", "Email"},
			{"total_gastado", "Total Gastado ($)"},
		}
		titulo = "Reporte de Clientes"
	case "empleados":
		data = ventasDB.ObtenerRendimientoEmpleadosPeriodo(ini, fin)
		columnas = []Columna{
			{"nombre_empleado", "Empleado"},
			{"cantidad_ventas", "Ventas"},
			{"total_facturado", "Total Facturado ($)"},
			{"ticket_promedio", "Ticket Promedio ($)"},
		}
		titulo = "Rendimiento de Empleados"
	default:
		jsonError(w, http.StatusBadRequest, "Tipo de reporte no válido")
		return
	}

	// Reemplazar cliente/empleado nulos (igual que la versión Python)
	for _, item := range data {
		if v, existe := item["nombre_cliente"]; existe {
			if s := strings.TrimSpace(database.AStr(v)); s == "" {
				item["nombre_cliente"] = "Consumidor Final"
			}
		}
		if v, existe := item["nombre_empleado"]; existe {
			if s := strings.TrimSpace(database.AStr(v)); s == "" {
				item["nombre_empleado"] = "N/A"
			}
		}
	}

	pdfBytes, err := crearPDFDesdeDatos(data, columnas, titulo, desde, hasta)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error al generar PDF: "+err.Error())
		return
	}

	fechaActual := time.Now().Format("20060102_150405")
	nombreArchivo := "reporte_" + tipo + "_" + fechaActual + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename="+nombreArchivo)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	_, _ = w.Write(pdfBytes)
}

// redondeo usado en reportes
var _ = math.Round
