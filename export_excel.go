package main

import (
	"bytes"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"mysys/database"
)

// crearExcelDesdeDatos genera un archivo Excel con encabezados estilizados,
// anchos de columna automáticos y bordes finos (equivalente a la versión
// Python con openpyxl/pandas).
func crearExcelDesdeDatos(data []map[string]any, columnas []Columna) (*bytes.Buffer, error) {
	f := excelize.NewFile()
	defer f.Close()

	hoja := "Reporte"
	indice, err := f.NewSheet(hoja)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(indice)

	if len(data) == 0 {
		_ = f.SetCellValue(hoja, "A1", "No hay datos disponibles para el período seleccionado.")
		buf, err := f.WriteToBuffer()
		return buf, err
	}

	// Estilo de encabezado: negrita blanca sobre #4F46E5, centrado, borde fino
	estiloHeader, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4F46E5"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}
	estiloCelda, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}

	// Filtrar columnas presentes en los datos, conservando el orden definido
	columnasMostrar := []Columna{}
	for _, c := range columnas {
		if _, existe := data[0][c.Clave]; existe {
			columnasMostrar = append(columnasMostrar, c)
		}
	}
	if len(columnasMostrar) == 0 {
		columnasMostrar = columnas
	}

	// Encabezados
	anchos := make([]int, len(columnasMostrar))
	for i, c := range columnasMostrar {
		celda, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(hoja, celda, c.Titulo)
		anchos[i] = utf8.RuneCountInString(c.Titulo)
	}
	ultimaCol, _ := excelize.ColumnNumberToName(len(columnasMostrar))
	_ = f.SetCellStyle(hoja, "A1", ultimaCol+"1", estiloHeader)

	// Datos
	for filaIdx, item := range data {
		fila := filaIdx + 2
		for colIdx, c := range columnasMostrar {
			celda, _ := excelize.CoordinatesToCellName(colIdx+1, fila)
			valor := item[c.Clave]
			switch v := valor.(type) {
			case nil:
				_ = f.SetCellValue(hoja, celda, "")
			case float64:
				_ = f.SetCellValue(hoja, celda, v)
			case int64:
				_ = f.SetCellValue(hoja, celda, v)
			default:
				s := database.AStr(valor)
				_ = f.SetCellValue(hoja, celda, s)
			}
			// Ancho automático (máx 50 caracteres)
			largo := utf8.RuneCountInString(fmt.Sprintf("%v", valor))
			if largo > anchos[colIdx] {
				anchos[colIdx] = largo
			}
		}
	}
	if len(data) > 0 {
		ultimaFila := len(data) + 1
		_ = f.SetCellStyle(hoja, "A2", fmt.Sprintf("%s%d", ultimaCol, ultimaFila), estiloCelda)
	}

	// Aplicar anchos calculados
	for i, ancho := range anchos {
		letra, _ := excelize.ColumnNumberToName(i + 1)
		ajustado := math.Min(float64(ancho)+2, 50)
		_ = f.SetColWidth(hoja, letra, letra, ajustado)
	}

	return f.WriteToBuffer()
}
