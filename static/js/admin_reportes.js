

/* Extracted from admin_reportes.html */

        // --- Variables para Chart.js ---
        let ventasChart = null;
        let metodosPagoChart = null;

        // --- Variable para almacenar datos actuales ---
        let datosActuales = null;
        let tipoReporteActual = 'ventas';

        // --- Tabs ---
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
                document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
                this.classList.add('active');
                document.getElementById(this.dataset.tab).classList.add('active');
            });
        });

        // --- Generar Reporte ---
        async function generarReporte() {
            const desde = document.getElementById('reporteFechaDesde').value;
            const hasta = document.getElementById('reporteFechaHasta').value;
            const tipo = document.getElementById('reporteTipo').value;
            tipoReporteActual = tipo;

            if (!desde || !hasta) {
                mostrarToast('Por favor selecciona ambas fechas.', 'warning');
                return;
            }

            try {
                const url = `/api/reportes/${tipo}?desde=${desde}&hasta=${hasta}`;
                const res = await fetch(url);
                if (!res.ok) throw new Error('Error al generar el reporte');
                const data = await res.json();
                datosActuales = data;

                // Actualizar fecha en el reporte
                const fechaLabel = document.getElementById(`fechaReporte${tipo.charAt(0).toUpperCase() + tipo.slice(1)}`);
                if (fechaLabel) {
                    fechaLabel.textContent = `📅 ${desde} al ${hasta}`;
                }

                // Renderizar según el tipo
                switch(tipo) {
                    case 'ventas':
                        renderizarReporteVentas(data);
                        break;
                    case 'auditoria':
                        renderizarReporteAuditoria(data);
                        break;
                    case 'productos':
                        renderizarReporteProductos(data);
                        break;
                    case 'clientes':
                        renderizarReporteClientes(data);
                        break;
                    case 'empleados':
                        renderizarReporteEmpleados(data);
                        break;
                }

                mostrarToast('Reporte generado exitosamente.', 'success');
            } catch (err) {
                mostrarToast('Error: ' + err.message, 'error');
            }
        }

        // --- Renderizar Reporte de Ventas ---
        function renderizarReporteVentas(data) {
            // Tabla
            const container = document.getElementById('tablaVentasContainer');
            if (!data.ventas || data.ventas.length === 0) {
                container.innerHTML = '<p style="padding: 2rem; text-align: center; color: var(--text-muted);">No hay ventas en el período seleccionado.</p>';
                return;
            }

            let html = `<table>
                <thead><tr>
                    <th>ID</th>
                    <th>Cliente</th>
                    <th>Empleado</th>
                    <th>Fecha</th>
                    <th>Método Pago</th>
                    <th>Total</th>
                </tr></thead><tbody>`;
            data.ventas.forEach(v => {
                html += `<tr>
                    <td>#${v.id}</td>
                    <td>${v.nombre_cliente || 'Consumidor Final'}</td>
                    <td>${v.nombre_empleado || 'N/A'}</td>
                    <td>${v.fecha_completa}</td>
                    <td>${v.metodo_pago}</td>
                    <td><strong class="currency-format">${v.total}</strong></td>
                </tr>`;
            });
            html += `</tbody></table>`;
            container.innerHTML = html;

            // Gráficos
            if (data.ventas_por_dia && data.ventas_por_dia.length > 0) {
                const labels = data.ventas_por_dia.map(d => d.dia);
                const totals = data.ventas_por_dia.map(d => d.total_recaudado || 0);

                const ctx1 = document.getElementById('ventasReporteChart').getContext('2d');
                if (ventasChart) ventasChart.destroy();
                ventasChart = new Chart(ctx1, {
                    type: 'bar',
                    data: {
                        labels: labels,
                        datasets: [{
                            label: 'Ventas por Día ($)',
                            data: totals,
                            backgroundColor: 'rgba(79, 70, 229, 0.7)',
                            borderColor: 'rgba(79, 70, 229, 1)',
                            borderWidth: 2,
                            borderRadius: 6,
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: { legend: { display: false } },
                        scales: {
                            y: { beginAtZero: true, ticks: { callback: v => '$' + v.toFixed(0) } }
                        }
                    }
                });
            }

            if (data.metodos_pago && data.metodos_pago.length > 0) {
                const labels = data.metodos_pago.map(m => m.metodo_pago);
                const counts = data.metodos_pago.map(m => m.cantidad || 0);
                const colores = ['#4f46e5', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6'];

                const ctx2 = document.getElementById('metodosPagoReporteChart').getContext('2d');
                if (metodosPagoChart) metodosPagoChart.destroy();
                metodosPagoChart = new Chart(ctx2, {
                    type: 'doughnut',
                    data: {
                        labels: labels,
                        datasets: [{
                            data: counts,
                            backgroundColor: colores.slice(0, labels.length),
                            borderWidth: 2,
                            borderColor: '#ffffff',
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: { position: 'bottom', labels: { padding: 12, usePointStyle: true } }
                        },
                        cutout: '60%'
                    }
                });
            }
        }

        // --- Renderizar Reporte de Auditoría ---
        function renderizarReporteAuditoria(data) {
            const container = document.getElementById('tablaAuditoriaContainer');
            if (!data.logs || data.logs.length === 0) {
                container.innerHTML = '<p style="padding: 2rem; text-align: center; color: var(--text-muted);">No hay registros de auditoría en el período seleccionado.</p>';
                return;
            }

            let html = `<table>
                <thead><tr>
                    <th>Fecha/Hora</th>
                    <th>Usuario</th>
                    <th>Acción</th>
                    <th>Tabla</th>
                    <th>Detalles</th>
                </tr></thead><tbody>`;
            data.logs.forEach(log => {
                const badgeClass = log.accion === 'LOGIN' ? 'badge-success' :
                                  log.accion === 'LOGOUT' ? 'badge-warning' :
                                  log.accion === 'INSERT' ? 'badge-info' :
                                  log.accion === 'UPDATE' ? 'badge-warning' :
                                  log.accion === 'DELETE' ? 'badge-danger' : '';
                html += `<tr>
                    <td>${log.fecha_hora}</td>
                    <td><strong>${log.usuario}</strong></td>
                    <td><span class="badge ${badgeClass}">${log.accion}</span></td>
                    <td>${log.tabla || '-'}</td>
                    <td>${log.detalles || '-'}</td>
                </tr>`;
            });
            html += `</tbody></table>`;
            container.innerHTML = html;
        }

        // --- Renderizar Reporte de Productos ---
        function renderizarReporteProductos(data) {
            const container = document.getElementById('tablaProductosContainer');
            if (!data.productos || data.productos.length === 0) {
                container.innerHTML = '<p style="padding: 2rem; text-align: center; color: var(--text-muted);">No hay productos registrados.</p>';
                return;
            }

            let html = `<table>
                <thead><tr>
                    <th>Nombre</th>
                    <th>Código</th>
                    <th>Proveedor</th>
                    <th>Categoría</th>
                    <th>Precio</th>
                    <th>Stock</th>
                </tr></thead><tbody>`;
            data.productos.forEach(p => {
                const stockBadge = p.stock === 0 ? 'badge-danger' : p.stock <= 5 ? 'badge-warning' : 'badge-success';
                html += `<tr>
                    <td><strong>${p.nombre}</strong></td>
                    <td>${p.codigo_barras}</td>
                    <td>${p.proveedor || '-'}</td>
                    <td>${p.categoria || '-'}</td>
                    <td class="currency-format">${p.precio}</td>
                    <td><span class="badge ${stockBadge}">${p.stock} uds</span></td>
                </tr>`;
            });
            html += `</tbody></table>`;
            container.innerHTML = html;
        }

        // --- Renderizar Reporte de Clientes ---
        function renderizarReporteClientes(data) {
            const container = document.getElementById('tablaClientesContainer');
            if (!data.clientes || data.clientes.length === 0) {
                container.innerHTML = '<p style="padding: 2rem; text-align: center; color: var(--text-muted);">No hay clientes registrados.</p>';
                return;
            }

            let html = `<table>
                <thead><tr>
                    <th>ID</th>
                    <th>Nombre</th>
                    <th>Identificación</th>
                    <th>Teléfono</th>
                    <th>Email</th>
                    <th>Total Gastado</th>
                </tr></thead><tbody>`;
            data.clientes.forEach(c => {
                html += `<tr>
                    <td>#${c.id}</td>
                    <td><strong>${c.nombre}</strong></td>
                    <td>${c.identificacion || '-'}</td>
                    <td>${c.telefono || '-'}</td>
                    <td>${c.email || '-'}</td>
                    <td class="currency-format">${c.total_gastado || '0.00'}</td>
                </tr>`;
            });
            html += `</tbody></table>`;
            container.innerHTML = html;
        }

        // --- Renderizar Reporte de Empleados ---
        function renderizarReporteEmpleados(data) {
            const container = document.getElementById('tablaEmpleadosContainer');
            if (!data.empleados || data.empleados.length === 0) {
                container.innerHTML = '<p style="padding: 2rem; text-align: center; color: var(--text-muted);">No hay empleados con ventas en el período.</p>';
                return;
            }

            let html = `<table>
                <thead><tr>
                    <th>#</th>
                    <th>Empleado</th>
                    <th>Ventas</th>
                    <th>Total Facturado</th>
                    <th>Ticket Promedio</th>
                </tr></thead><tbody>`;
            data.empleados.forEach((e, i) => {
                html += `<tr>
                    <td>${i + 1}</td>
                    <td><strong>${e.nombre_empleado}</strong></td>
                    <td>${e.cantidad_ventas}</td>
                    <td class="currency-format">${e.total_facturado}</td>
                    <td class="currency-format">${e.ticket_promedio || '0.00'}</td>
                </tr>`;
            });
            html += `</tbody></table>`;
            container.innerHTML = html;
        }

        // ==========================================
        //   EXPORTAR A EXCEL
        // ==========================================
        async function exportarExcel() {
            const desde = document.getElementById('reporteFechaDesde').value;
            const hasta = document.getElementById('reporteFechaHasta').value;
            const tipo = document.getElementById('reporteTipo').value;

            if (!desde || !hasta) {
                mostrarToast('Por favor selecciona ambas fechas.', 'warning');
                return;
            }

            // Mapeo de tipos a endpoints
            const endpoints = {
                'ventas': '/api/reportes/exportar/ventas',
                'auditoria': '/api/reportes/exportar/auditoria',
                'productos': '/api/reportes/exportar/productos',
                'clientes': '/api/reportes/exportar/clientes',
                'empleados': '/api/reportes/exportar/empleados'
            };

            const url = endpoints[tipo];
            if (!url) {
                mostrarToast('Tipo de reporte no válido.', 'error');
                return;
            }

            try {
                mostrarToast('Generando archivo Excel...', 'info');
                
                const params = new URLSearchParams();
                params.append('desde', desde);
                params.append('hasta', hasta);
                
                const response = await fetch(`${url}?${params.toString()}`, {
                    method: 'GET',
                    credentials: 'include'
                });

                if (!response.ok) {
                    const error = await response.json();
                    throw new Error(error.detail || 'Error al generar el Excel');
                }

                const blob = await response.blob();
                const link = document.createElement('a');
                link.href = URL.createObjectURL(blob);
                const contentDisposition = response.headers.get('content-disposition');
                const filename = contentDisposition ? contentDisposition.split('filename=')[1].replace(/"/g, '') : `reporte_${tipo}.xlsx`;
                link.download = filename;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                URL.revokeObjectURL(link.href);
                
                mostrarToast('✅ Excel exportado correctamente.', 'success');
            } catch (error) {
                console.error('Error exportando Excel:', error);
                mostrarToast('❌ Error: ' + error.message, 'error');
            }
        }

        // ==========================================
        //   EXPORTAR A PDF
        // ==========================================

        async function exportarPDF() {
            const desde = document.getElementById('reporteFechaDesde').value;
            const hasta = document.getElementById('reporteFechaHasta').value;
            const tipo = document.getElementById('reporteTipo').value;

            if (!desde || !hasta) {
                mostrarToast('⚠️ Por favor selecciona ambas fechas.', 'warning');
                return;
            }

            // Verificar si hay datos en la tabla
            const activeTab = document.querySelector('.tab-content.active');
            const tabla = activeTab.querySelector('table');
            if (!tabla || tabla.querySelectorAll('tbody tr').length === 0) {
                mostrarToast('⚠️ No hay datos para exportar a PDF.', 'warning');
                return;
            }

            // Mostrar indicador de carga
            const btnPDF = document.querySelector('.btn-danger');
            const originalText = btnPDF.innerHTML;
            btnPDF.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Generando...';
            btnPDF.disabled = true;

            try {
                const params = new URLSearchParams();
                params.append('desde', desde);
                params.append('hasta', hasta);
                params.append('tipo', tipo);
                
                mostrarToast('📄 Generando PDF, por favor espera...', 'info');
                
                const response = await fetch(`/api/reportes/exportar/pdf?${params.toString()}`, {
                    method: 'GET',
                    credentials: 'include'
                });

                if (!response.ok) {
                    const error = await response.json();
                    throw new Error(error.detail || 'Error al generar el PDF');
                }

                // Descargar el archivo
                const blob = await response.blob();
                const link = document.createElement('a');
                link.href = URL.createObjectURL(blob);
                const contentDisposition = response.headers.get('content-disposition');
                const filename = contentDisposition ? contentDisposition.split('filename=')[1].replace(/"/g, '') : `reporte_${tipo}.pdf`;
                link.download = filename;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                URL.revokeObjectURL(link.href);
                
                mostrarToast('✅ PDF exportado correctamente.', 'success');
            } catch (error) {
                console.error('Error exportando PDF:', error);
                mostrarToast('❌ Error: ' + error.message, 'error');
            } finally {
                btnPDF.innerHTML = originalText;
                btnPDF.disabled = false;
            }
        }

        // ==========================================
        //   EXPORTAR CSV
        // ==========================================
        function exportarCSV() {
            const activeTab = document.querySelector('.tab-content.active');
            const table = activeTab.querySelector('table');
            if (!table) {
                mostrarToast('No hay datos para exportar.', 'warning');
                return;
            }

            let csv = '';
            const headers = table.querySelectorAll('thead th');
            const headerRow = Array.from(headers).map(th => th.textContent.trim()).join(',');
            csv += headerRow + '\n';

            const rows = table.querySelectorAll('tbody tr');
            rows.forEach(row => {
                const cells = row.querySelectorAll('td');
                const rowData = Array.from(cells).map(td => {
                    let text = td.textContent.trim();
                    text = text.replace(/,/g, ';');
                    text = text.replace(/\n/g, ' ');
                    return text;
                }).join(',');
                csv += rowData + '\n';
            });

            const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            const fecha = new Date().toISOString().slice(0,10);
            a.download = `reporte_${document.getElementById('reporteTipo').value}_${fecha}.csv`;
            a.click();
            URL.revokeObjectURL(url);
            mostrarToast('CSV exportado correctamente.', 'success');
        }

        // ==========================================
        //   TOAST / NOTIFICACIONES
        // ==========================================
        function mostrarToast(mensaje, tipo = 'info') {
            const toast = document.createElement('div');
            const colores = {
                success: '#10b981',
                error: '#ef4444',
                warning: '#f59e0b',
                info: '#4f46e5'
            };
            toast.style.cssText = `
                position: fixed;
                bottom: 20px;
                right: 20px;
                padding: 12px 20px;
                border-radius: 8px;
                color: white;
                font-weight: 600;
                font-size: 0.9rem;
                z-index: 9999;
                background: ${colores[tipo] || colores.info};
                box-shadow: 0 4px 12px rgba(0,0,0,0.15);
                animation: slideIn 0.3s ease;
                max-width: 400px;
            `;
            toast.textContent = mensaje;
            document.body.appendChild(toast);
            setTimeout(() => {
                toast.style.opacity = '0';
                toast.style.transition = 'opacity 0.3s';
                setTimeout(() => toast.remove(), 300);
            }, 4000);
        }

        // --- Agregar animación de entrada para toast ---
        const style = document.createElement('style');
        style.textContent = `
            @keyframes slideIn {
                from { transform: translateX(100px); opacity: 0; }
                to { transform: translateX(0); opacity: 1; }
            }
        `;
        document.head.appendChild(style);

        // ==========================================
        //   INICIALIZACIÓN
        // ==========================================
        document.addEventListener('DOMContentLoaded', function() {
            const hoy = new Date();
            const hace30Dias = new Date();
            hace30Dias.setDate(hoy.getDate() - 30);
            document.getElementById('reporteFechaDesde').value = hace30Dias.toISOString().slice(0,10);
            document.getElementById('reporteFechaHasta').value = hoy.toISOString().slice(0,10);

            setTimeout(generarReporte, 300);
        });
    
