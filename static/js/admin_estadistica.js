document.addEventListener('DOMContentLoaded', function() {
    const statsDataElement = document.getElementById('statsData');
    if (!statsDataElement) {
        console.warn("No se encontró el bloque de datos de estadísticas.");
        return;
    }
    
    let statsData;
    try {
        statsData = JSON.parse(statsDataElement.textContent);
    } catch (e) {
        console.error("Error al parsear los datos de estadísticas:", e);
        return;
    }

    const colores = ['#4f46e5', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899'];

    // --- Gráfica de Ventas por Día ---
    const ctx1 = document.getElementById('ventasDiariasChart');
    if (ctx1 && statsData.ventas_por_dia) {
        new Chart(ctx1, {
            type: 'bar',
            data: {
                labels: statsData.ventas_por_dia.map(item => item.dia || 'Sin fecha'),
                datasets: [{
                    label: 'Total Recaudado ($)',
                    data: statsData.ventas_por_dia.map(item => item.total_recaudado || 0),
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
                    y: {
                        beginAtZero: true,
                        ticks: { callback: function(value) { return '$' + value.toLocaleString(); } }
                    }
                }
            }
        });
    }

    // --- Gráfica de Métodos de Pago ---
    const ctx2 = document.getElementById('metodosPagoChart');
    if (ctx2 && statsData.metodos_pago) {
        new Chart(ctx2, {
            type: 'doughnut',
            data: {
                labels: statsData.metodos_pago.map(item => item.metodo_pago || 'Otro'),
                datasets: [{
                    data: statsData.metodos_pago.map(item => item.cantidad || 0),
                    backgroundColor: colores.slice(0, statsData.metodos_pago.length),
                    borderWidth: 2,
                    borderColor: '#ffffff',
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { position: 'bottom', labels: { padding: 12, usePointStyle: true, pointStyle: 'circle' } }
                },
                cutout: '60%'
            }
        });
    }

    // --- Gráfica de Top 5 Productos ---
    const ctx3 = document.getElementById('topProductosChart');
    if (ctx3 && statsData.top_productos) {
        new Chart(ctx3, {
            type: 'bar',
            data: {
                labels: statsData.top_productos.map(item => item.nombre_producto || 'Producto'),
                datasets: [{
                    label: 'Unidades Vendidas',
                    data: statsData.top_productos.map(item => item.cantidad_vendida || 0),
                    backgroundColor: 'rgba(16, 185, 129, 0.7)',
                    borderColor: 'rgba(16, 185, 129, 1)',
                    borderWidth: 2,
                    borderRadius: 6,
                }]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: { x: { beginAtZero: true, ticks: { stepSize: 1 } } }
            }
        });
    }

    // --- Gráfica de Top 5 Clientes ---
    const ctx4 = document.getElementById('topClientesChart');
    if (ctx4 && statsData.top_clientes) {
        new Chart(ctx4, {
            type: 'bar',
            data: {
                labels: statsData.top_clientes.map(item => item.nombre_cliente || 'Cliente'),
                datasets: [{
                    label: 'Total Gastado ($)',
                    data: statsData.top_clientes.map(item => item.total_gastado || 0),
                    backgroundColor: 'rgba(239, 68, 68, 0.7)',
                    borderColor: 'rgba(239, 68, 68, 1)',
                    borderWidth: 2,
                    borderRadius: 6,
                }]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: {
                        beginAtZero: true,
                        ticks: { callback: function(value) { return '$' + value.toLocaleString(); } }
                    }
                }
            }
        });
    }
});