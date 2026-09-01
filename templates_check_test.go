package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTemplatesCompile parsea y EJECUTA todas las plantillas .html con un
// contexto de muestra, para detectar errores de sintaxis y de ejecución.
// go test -run TestTemplatesCompile -v
func TestTemplatesCompile(t *testing.T) {
	initTemplates()
	if tpl == nil {
		t.Fatal("no se cargaron las plantillas")
	}
	ahora := time.Date(2026, 8, 30, 13, 31, 0, 0, time.Local)
	muestra := map[string]any{
		"username": "admin", "rol": "admin", "foto": nil, "error": "x", "success": "y",
		"total_hoy": 6.9, "total_historico": 100.5,
		"ultimas_ventas":          []map[string]any{},
		"ventas_por_dia":          []map[string]any{{"dia": "2026-08-30", "cantidad_ventas": 2, "total_recaudado": 6.9}},
		"productos":               []map[string]any{{"id": 1, "nombre": "P", "precio": 1.5, "iva": 15.0, "stock": 5, "categoria": "C", "proveedor": "PR", "codigo_barras": "750", "imagen_url": nil}},
		"clientes":                []map[string]any{{"id": 1, "nombre": "Juan", "tipo_identificacion": "cedula", "identificacion": "091", "telefono": "099", "email": nil, "direccion": nil}},
		"logs_auditoria":          []map[string]any{{"usuario": "admin", "usuario_id": 1, "accion": "LOGIN", "tabla": "users", "registro_id": 1, "detalles": "x", "fecha_hora": "2026-08-30 13:00:00"}},
		"usuarios":                [][]any{{"admin", "admin", "450", "Gerente", "Admin"}},
		"archivos":                []map[string]any{},
		"tamano_total_formateado": "0 B", "total_usuarios_unicos": 0,
		"ventas_con_productos": []map[string]any{{
			"id": 1, "total": 3.44, "metodo_pago": "Efectivo", "estado": "Completada",
			"fecha_completa": "2026-08-30 13:31:06", "fecha_completa_dt": ahora,
			"nombre_cliente": "Juan", "nombre_empleado": "admin",
			"productos": []map[string]any{{"cantidad": 2, "precio_unitario": 1.5, "iva": 15.0, "subtotal": 3.44, "nombre_producto": "P"}},
		}},
		"ventas":           []map[string]any{},
		"metodos_pago":     []map[string]any{{"metodo_pago": "Efectivo", "cantidad": 1, "total_recaudado": 3.44}},
		"top_productos":    []map[string]any{{"nombre_producto": "P", "cantidad_vendida": 2, "total_generado": 3.44}},
		"top_clientes":     []map[string]any{{"nombre_cliente": "Juan", "cantidad_compras": 1, "total_gastado": 3.44}},
		"empleados_ventas": []map[string]any{{"nombre_empleado": "admin", "cantidad_ventas": 1, "total_facturado": 3.44, "ticket_promedio": 3.44, "porcentaje": 100.0}},
		"total_periodo":    6.9, "total_ventas_periodo": 2, "ticket_promedio": 3.45,
		"productos_vendidos": 4, "error_filtro": nil,
		"filtro":        map[string]any{"desde": "2026-08-01", "hasta": "2026-08-30", "periodo": 30},
		"stats_payload": map[string]any{"ventas_por_dia": []map[string]any{}, "metodos_pago": []map[string]any{}, "top_productos": []map[string]any{}, "top_clientes": []map[string]any{}},
		"usuario":       "admin",
		"perfil":        map[string]any{"username": "admin", "nombre": "Admin", "puesto": "Gerente", "tipo": "admin", "foto": nil},
		"server_ip":     "127.0.0.1", "server_port": 8000,
		"request": map[string]any{"url": map[string]any{"path": "/", "port": 8000}, "query_params": map[string]string{}},
		"detail":  "",
	}
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".html" {
			continue
		}
		plantilla := tpl.Lookup(e.Name())
		if plantilla == nil {
			t.Errorf("%s: no registrada", e.Name())
			continue
		}
		var buf bytes.Buffer
		if err := plantilla.Execute(&buf, muestra); err != nil {
			t.Errorf("%s: %v", e.Name(), err)
		}
	}
}
