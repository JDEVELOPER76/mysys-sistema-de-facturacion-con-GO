package main

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"

	"mysys/database"
)

// aCP1252 convierte UTF-8 a CP1252/Latin-1 para las fuentes base del PDF.
// Los caracteres del español (áéíóúñü¿¡) coinciden en ambos conjuntos.
func aCP1252(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch {
		case r < 256:
			b.WriteByte(byte(r))
		case r == '€':
			b.WriteByte(0x80)
		case r == '—':
			b.WriteByte(0x97)
		case r == '–':
			b.WriteByte(0x96)
		case r == '“' || r == '”':
			b.WriteByte('"')
		case r == '‘' || r == '’':
			b.WriteByte('\'')
		case r == '…':
			b.WriteString("...")
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

// calcularAnchosColumnas calcula el ancho óptimo por columna (6 pt/carácter + 20, máx 120).
func calcularAnchosColumnas(data []map[string]any, columnas []Columna, maxWidth float64) []float64 {
	if len(columnas) == 0 {
		return nil
	}
	anchos := make([]float64, len(columnas))
	for i, c := range columnas {
		maxLen := len(c.Titulo)
		for _, item := range data {
			v := item[c.Clave]
			var s string
			switch t := v.(type) {
			case nil:
				s = ""
			case float64:
				s = fmt.Sprintf("%.2f", t)
			default:
				s = fmt.Sprintf("%v", v)
			}
			if len(s) > 50 {
				s = s[:50]
			}
			if len(s) > maxLen {
				maxLen = len(s)
			}
		}
		anchos[i] = math.Min(float64(maxLen)*6+20, maxWidth)
	}
	return anchos
}

// crearPDFDesdeDatos genera un PDF tamaño carta con la tabla del reporte
// (equivalente a la versión Python con reportlab).
func crearPDFDesdeDatos(data []map[string]any, columnas []Columna, titulo, desde, hasta string) ([]byte, error) {
	pdf := gofpdf.New("P", "pt", "Letter", "")
	pdf.SetMargins(72, 72, 72)
	pdf.SetAutoPageBreak(true, 72)
	pdf.AddPage()

	// Título
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetTextColor(30, 41, 59) // #1e293b
	pdf.CellFormat(0, 22, aCP1252("MySYS - Sistema de Facturación"), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// Subtítulos
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(107, 114, 128) // #6b7280
	pdf.CellFormat(0, 12, aCP1252(titulo), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 12, aCP1252(fmt.Sprintf("Período: %s al %s", desde, hasta)), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 12, aCP1252("Generado: "+time.Now().Format("02/01/2006 15:04:05")), "", 1, "C", false, 0, "")
	pdf.Ln(14)

	escribirMensaje := func(msg string) {
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(31, 41, 55)
		pdf.CellFormat(0, 10, aCP1252(msg), "", 1, "L", false, 0, "")
	}

	if len(data) == 0 || len(columnas) == 0 {
		if len(columnas) == 0 {
			escribirMensaje("Error: No se definieron columnas para el reporte.")
		} else {
			escribirMensaje("No hay datos disponibles para el período seleccionado.")
		}
		return finalizarPDF(pdf)
	}

	// Encabezados (truncados a 20 caracteres)
	headers := make([]string, len(columnas))
	for i, c := range columnas {
		h := c.Titulo
		if len(h) > 20 {
			h = h[:18] + "..."
		}
		headers[i] = aCP1252(h)
	}

	// Filas de datos
	filas := make([][]string, 0, len(data))
	for _, item := range data {
		fila := make([]string, len(columnas))
		for j, c := range columnas {
			v := item[c.Clave]
			var s string
			switch t := v.(type) {
			case nil:
				s = ""
			case float64:
				s = fmt.Sprintf("$%.2f", t)
			default:
				s = fmt.Sprintf("%v", v)
			}
			if len(s) > 40 {
				s = s[:37] + "..."
			}
			fila[j] = aCP1252(s)
		}
		filas = append(filas, fila)
	}

	if len(filas) == 0 {
		escribirMensaje("No hay datos disponibles para el período seleccionado.")
		return finalizarPDF(pdf)
	}

	anchos := calcularAnchosColumnas(data, columnas, 120)

	// Dibujar la tabla
	fontSize := 8.0
	if len(filas) > 30 {
		fontSize = 7
	}
	alturaFila := fontSize + 8 // padding vertical ~5pt arriba y abajo

	dibujarEncabezado := func() {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(79, 70, 229) // #4f46e5
		pdf.SetTextColor(255, 255, 255)
		pdf.SetDrawColor(229, 231, 235) // #e5e7eb
		pdf.SetLineWidth(0.5)
		for i, h := range headers {
			pdf.CellFormat(anchos[i], alturaFila, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}

	dibujarEncabezado()

	pdf.SetFont("Helvetica", "", fontSize)
	for idx, fila := range filas {
		if pdf.GetY()+alturaFila > 792-72 { // alto carta - margen inferior
			pdf.AddPage()
			dibujarEncabezado()
			pdf.SetFont("Helvetica", "", fontSize)
		}
		// Filas alternas #f8fafc
		relleno := idx%2 == 0
		if relleno {
			pdf.SetFillColor(248, 250, 252)
		}
		pdf.SetTextColor(31, 41, 55) // #1f2937
		for j, celda := range fila {
			pdf.CellFormat(anchos[j], alturaFila, celda, "1", 0, "L", relleno, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.Ln(8)

	// Resumen: total general si hay columna 'total'
	tieneTotal := false
	for _, c := range columnas {
		if c.Clave == "total" {
			tieneTotal = true
			break
		}
	}
	if tieneTotal {
		totalGeneral := 0.0
		for _, item := range data {
			totalGeneral += database.AFloat(item["total"])
		}
		if totalGeneral > 0 {
			pdf.SetFont("Helvetica", "B", 10)
			pdf.SetTextColor(30, 41, 59)
			pdf.CellFormat(0, 14, aCP1252(fmt.Sprintf("Total General: $%.2f", totalGeneral)), "", 1, "R", false, 0, "")
			pdf.Ln(4)
		}
	}

	// Pie de página
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(107, 114, 128)
	pdf.CellFormat(0, 10, aCP1252("Reporte generado automáticamente por MySYS"), "", 1, "C", false, 0, "")

	return finalizarPDF(pdf)
}

func finalizarPDF(pdf *gofpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		// Si hay error, generar un PDF simple con el mensaje (como la versión Python)
		pdf2 := gofpdf.New("P", "pt", "Letter", "")
		pdf2.SetMargins(72, 72, 72)
		pdf2.AddPage()
		pdf2.SetFont("Helvetica", "", 12)
		pdf2.SetTextColor(239, 68, 68) // #ef4444
		pdf2.CellFormat(0, 14, aCP1252("Error al generar el PDF: "+err.Error()), "", 1, "L", false, 0, "")
		buf.Reset()
		if err2 := pdf2.Output(&buf); err2 != nil {
			return nil, err2
		}
	}
	return buf.Bytes(), nil
}

var _ = strings.TrimSpace
