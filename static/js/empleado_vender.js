// ==========================================
//   EMPLEADO_VENDER.JS - PUNTO DE VENTA
// ==========================================

let carrito = [];
let scannerConnected = false;

// ==========================================
//   NOTIFICACIONES
// ==========================================
function mostrarNotificacion(mensaje, tipo = 'success') {
    const notificacion = document.createElement('div');
    notificacion.className = `notification ${tipo}`;
    const icono = tipo === 'success' ? 'fa-check-circle' : 
                  tipo === 'error' ? 'fa-exclamation-circle' : 
                  tipo === 'warning' ? 'fa-triangle-exclamation' : 'fa-info-circle';
    notificacion.innerHTML = `<i class="fa-solid ${icono}"></i> ${mensaje}`;
    document.body.appendChild(notificacion);
    
    setTimeout(() => {
        notificacion.style.animation = 'slideOut 0.3s ease';
        setTimeout(() => notificacion.remove(), 300);
    }, 3500);
}

// ==========================================
//   ESTADO DEL ESCÁNER
// ==========================================
function actualizarEstadoScanner(conectado, usuario = null) {
    const statusElement = document.getElementById('scannerStatus');
    const statusText = document.getElementById('scannerStatusText');
    if (conectado && usuario) {
        statusElement.className = 'scanner-status connected';
        statusText.textContent = `Escáner activo: ${usuario}`;
        scannerConnected = true;
    } else {
        statusElement.className = 'scanner-status';
        statusText.textContent = 'Esperando escáner...';
        scannerConnected = false;
    }
}

// ==========================================
//   VERIFICAR ESCÁNER Y PROCESAR LECTURAS
// ==========================================
async function verificarScannerActivo() {
    try {
        const response = await fetch('/api/verificar_lecturas', {
            credentials: 'include',
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
        });
        
        if (response.ok) {
            const data = await response.json();
            
            if (data.conectado && data.usuario) {
                actualizarEstadoScanner(true, data.usuario);
            } else {
                actualizarEstadoScanner(false);
            }
            
            if (data.codigo) {
                mostrarNotificacion(`Procesando código: ${data.codigo}`, 'info');
                
                const productoResponse = await fetch('/api/leer_codigo', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ 
                        codigo_barras: data.codigo,
                        usuario: data.usuario 
                    })
                });
                
                if (productoResponse.ok) {
                    const producto = await productoResponse.json();
                    
                    if (producto.stock > 0) {
                        agregarAlCarrito(producto.id, producto.nombre, producto.precio, producto.iva || 0);
                        mostrarNotificacion(`✅ ${producto.nombre} agregado al carrito`, 'success');
                    } else {
                        mostrarNotificacion(`⚠️ ${producto.nombre} - Sin stock disponible`, 'error');
                    }
                } else {
                    const error = await productoResponse.json();
                    mostrarNotificacion(`❌ ${error.detail || 'Código no reconocido'}`, 'error');
                }
            }
        }
    } catch (error) {
        actualizarEstadoScanner(false);
    }
}

// ==========================================
//   ABRIR ESCÁNER
// ==========================================
function abrirScanner() {
    const scannerWindow = window.open('/code', 'HadroxScanner', 'width=500,height=600,toolbar=no,menubar=no');
    if (scannerWindow) {
        mostrarNotificacion('🖨️ Ventana del escáner abierta. Escanea productos para agregarlos automáticamente.', 'info');
        
        scannerWindow.addEventListener('load', () => {
            scannerWindow.postMessage({ type: 'CONECTAR_TERMINAL', origen: 'terminal' }, '*');
        });
        
        const messageHandler = (event) => {
            if (event.data && event.data.type === 'PRODUCTO_ESCANEADO') {
                const producto = event.data.producto;
                if (producto && producto.id) {
                    agregarAlCarrito(producto.id, producto.nombre, producto.precio, producto.iva || 0);
                    mostrarNotificacion(`✅ ${producto.nombre} agregado (Scanner)`, 'success');
                }
            }
        };
        window.addEventListener('message', messageHandler);
        
        const checkClosed = setInterval(() => {
            if (scannerWindow.closed) {
                clearInterval(checkClosed);
                window.removeEventListener('message', messageHandler);
                mostrarNotificacion('🔌 Ventana del escáner cerrada', 'info');
                actualizarEstadoScanner(false);
            }
        }, 1000);
    } else {
        mostrarNotificacion('⚠️ No se pudo abrir el escáner. Permite ventanas emergentes.', 'error');
    }
}

// ==========================================
//   AGREGAR AL CARRITO
// ==========================================
function agregarAlCarrito(id, nombre, precio, iva) {
    // Validar datos
    const idNum = parseInt(id);
    const precioNum = parseFloat(precio);
    const ivaNum = parseFloat(iva || 0);
    
    if (isNaN(idNum) || isNaN(precioNum) || isNaN(ivaNum)) {
        mostrarNotificacion('⚠️ Datos del producto inválidos', 'error');
        return;
    }
    
    const itemExistente = carrito.find(item => item.producto_id === idNum);
    if (itemExistente) {
        itemExistente.cantidad += 1;
        mostrarNotificacion(`✅ ${nombre} x${itemExistente.cantidad}`, 'success');
    } else {
        carrito.push({
            producto_id: idNum,
            nombre: nombre,
            precio_unitario: precioNum,
            cantidad: 1,
            iva: ivaNum
        });
        mostrarNotificacion(`✅ ${nombre} agregado al carrito`, 'success');
    }
    renderizarCarrito();
}

// ==========================================
//   REMOVER DEL CARRITO
// ==========================================
function removerDelCarrito(id) {
    const item = carrito.find(item => item.producto_id === id);
    if (item) {
        carrito = carrito.filter(item => item.producto_id !== id);
        mostrarNotificacion(`🗑️ ${item.nombre} eliminado`, 'info');
        renderizarCarrito();
    }
}

// ==========================================
//   RENDERIZAR CARRITO
// ==========================================
function renderizarCarrito() {
    const container = document.getElementById('cart-container');
    container.innerHTML = '';

    if (carrito.length === 0) {
        container.innerHTML = `
            <p style="text-align:center; color: var(--hadrox-light); font-size:13px; margin-top:40px; font-weight:500;">
                <i class="fa-solid fa-cart-shopping" style="font-size:24px; display:block; margin-bottom:10px; color:var(--hadrox-border);"></i>
                El carrito de compras está vacío.
            </p>
        `;
        actualizarPanelTotales(0);
        document.getElementById('cart-count').textContent = '0';
        return;
    }

    let totalFinal = 0;

    carrito.forEach(item => {
        const precioConIva = item.precio_unitario * (1 + item.iva / 100);
        const subtotalItem = precioConIva * item.cantidad;
        totalFinal += subtotalItem;

        const itemDiv = document.createElement('div');
        itemDiv.className = 'cart-item';
        itemDiv.innerHTML = `
            <div class="item-details">
                <h5>${item.nombre}</h5>
                <p>$${item.precio_unitario.toFixed(2)} c/u 
                    ${item.iva > 0 ? `<small style="color:var(--hadrox-blue); font-weight:bold;">(IVA ${item.iva}%)</small>` : ''}
                    <span style="color:var(--hadrox-light); font-size:11px;">x${item.cantidad}</span>
                </p>
                <p style="font-weight:700; color:var(--hadrox-navy);">$${subtotalItem.toFixed(2)}</p>
            </div>
            <div class="item-controls">
                <button class="btn-qty" onclick="cambiarCantidad(${item.producto_id}, -1)">
                    <i class="fa-solid fa-minus"></i>
                </button>
                <span class="quantity-badge">${item.cantidad}</span>
                <button class="btn-qty" onclick="cambiarCantidad(${item.producto_id}, 1)">
                    <i class="fa-solid fa-plus"></i>
                </button>
                <button class="btn-remove" onclick="removerDelCarrito(${item.producto_id})">
                    <i class="fa-solid fa-trash-can"></i>
                </button>
            </div>
        `;
        container.appendChild(itemDiv);
    });

    actualizarPanelTotales(totalFinal);
    document.getElementById('cart-count').textContent = carrito.reduce((sum, item) => sum + item.cantidad, 0);
}

// ==========================================
//   CAMBIAR CANTIDAD
// ==========================================
function cambiarCantidad(productoId, delta) {
    const item = carrito.find(item => item.producto_id === productoId);
    if (!item) return;
    
    item.cantidad += delta;
    if (item.cantidad <= 0) {
        carrito = carrito.filter(i => i.producto_id !== productoId);
        mostrarNotificacion(`🗑️ ${item.nombre} eliminado`, 'info');
    }
    renderizarCarrito();
}

// ==========================================
//   ACTUALIZAR TOTALES
// ==========================================
function actualizarPanelTotales(total) {
    document.getElementById('grand-total').textContent = `$${total.toFixed(2)}`;
}

// ==========================================
//   FILTRAR PRODUCTOS
// ==========================================
function filtrarProductos() {
    const query = document.getElementById('search-input').value.toLowerCase().trim();
    document.querySelectorAll('.product-card').forEach(card => {
        const nombre = card.getAttribute('data-nombre') || '';
        card.style.display = nombre.includes(query) ? 'flex' : 'none';
    });
}

// ==========================================
//   CONFIRMAR VENTA (CORREGIDO)
// ==========================================
function confirmarVenta() {
    if (carrito.length === 0) {
        mostrarNotificacion('⚠️ Por favor, añade al menos un artículo al carrito.', 'warning');
        return;
    }

    const totalElement = document.getElementById('grand-total');
    const totalAmount = totalElement.textContent;
    const metodoPago = document.getElementById('pago-select').value;
    const clienteSelect = document.getElementById('cliente-select');
    const clienteText = clienteSelect.options[clienteSelect.selectedIndex].text;

    mostrarModalConfirmacion(totalAmount, metodoPago, clienteText);
}

// ==========================================
//   PROCESAR VENTA (CORREGIDO)
// ==========================================
async function procesarVenta() {
    if (carrito.length === 0) {
        mostrarNotificacion('⚠️ Por favor, añade al menos un artículo al carrito.', 'warning');
        return;
    }
    
    // Obtener cliente_id
    const clienteSelect = document.getElementById('cliente-select');
    const clienteId = clienteSelect.value ? parseInt(clienteSelect.value) : null;
    
    // Construir payload CORRECTO
    const payload = {
        cliente_id: clienteId,
        metodo_pago: document.getElementById('pago-select').value,
        detalles: carrito.map(item => ({
            producto_id: item.producto_id,
            cantidad: item.cantidad,
            precio_unitario: item.precio_unitario,
            iva: item.iva
        }))
    };

    // Log para depuración
    console.log('📤 Enviando payload:', JSON.stringify(payload, null, 2));

    const btnCheckout = document.querySelector('.btn-checkout');
    const originalText = btnCheckout.innerHTML;
    btnCheckout.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Procesando...';
    btnCheckout.disabled = true;

    try {
        const response = await fetch('/api/vender', {
            method: 'POST',
            headers: { 
                'Content-Type': 'application/json',
                'Accept': 'application/json'
            },
            body: JSON.stringify(payload),
            credentials: 'include'
        });

        // Leer la respuesta como texto para depuración
        const responseText = await response.text();
        console.log('📥 Respuesta del servidor:', responseText);

        let data;
        try {
            data = JSON.parse(responseText);
        } catch (e) {
            throw new Error('El servidor respondió con un formato inválido: ' + responseText.substring(0, 200));
        }

        if (!response.ok) {
            throw new Error(data.detail || `Error ${response.status}: ${response.statusText}`);
        }

        if (data.success) {
            mostrarNotificacion(`🎉 ¡Venta #${data.venta_id} registrada correctamente!`, 'success');
            carrito = [];
            renderizarCarrito();
            document.getElementById('cart-count').textContent = '0';
        } else {
            mostrarNotificacion('❌ Error en cobro: ' + (data.detail || 'No se pudo procesar la venta.'), 'error');
        }
    } catch (err) {
        console.error('❌ Error al procesar venta:', err);
        mostrarNotificacion('❌ ' + err.message, 'error');
    } finally {
        btnCheckout.innerHTML = originalText;
        btnCheckout.disabled = false;
    }
}

// ==========================================
//   MODALES
// ==========================================

function mostrarModalConfirmacion(total, metodo, cliente) {
    document.getElementById('modal-total').textContent = total;
    document.getElementById('modal-metodo').textContent = metodo;
    document.getElementById('modal-cliente').textContent = cliente;
    document.getElementById('modal-overlay').classList.add('active');
}

function cerrarModal() {
    document.getElementById('modal-overlay').classList.remove('active');
}

function confirmarOperacion() {
    cerrarModal();
    procesarVenta();
}

// ==========================================
//   MANEJO DEL MODAL DE CLIENTE
// ==========================================

const modalCliente = document.getElementById('modalCliente');
const btnAgregarCliente = document.getElementById('btnAgregarCliente');
const btnCancelarCliente = document.getElementById('cancelarCliente');
const cerrarModalCliente = document.getElementById('cerrarModalCliente');
const formNuevoCliente = document.getElementById('formNuevoCliente');

function abrirModalCliente() {
    modalCliente.classList.add('active');
    setTimeout(() => {
        document.getElementById('inputClienteNombre').focus();
    }, 100);
}

function cerrarModalClienteFn() {
    modalCliente.classList.remove('active');
    formNuevoCliente.reset();
    const btnGuardar = document.getElementById('btnGuardarCliente');
    btnGuardar.innerHTML = '<i class="fa-solid fa-check"></i> Guardar Cliente';
    btnGuardar.disabled = false;
}

if (btnAgregarCliente) {
    btnAgregarCliente.addEventListener('click', abrirModalCliente);
}

if (btnCancelarCliente) {
    btnCancelarCliente.addEventListener('click', cerrarModalClienteFn);
}

if (cerrarModalCliente) {
    cerrarModalCliente.addEventListener('click', cerrarModalClienteFn);
}

modalCliente.addEventListener('click', (e) => {
    if (e.target === modalCliente) {
        cerrarModalClienteFn();
    }
});

// ==========================================
//   FORMULARIO NUEVO CLIENTE
// ==========================================
formNuevoCliente.addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const nombre = document.getElementById('inputClienteNombre').value.trim();
    const tipo_identificacion = document.getElementById('inputClienteTipoID').value;
    const identificacion = document.getElementById('inputClienteID').value.trim();
    const telefono = document.getElementById('inputClienteTelefono').value.trim();
    const email = document.getElementById('inputClienteEmail').value.trim();
    const direccion = document.getElementById('inputClienteDireccion').value.trim();
    
    if (!nombre) {
        mostrarNotificacion('⚠️ Por favor ingresa el nombre del cliente', 'warning');
        document.getElementById('inputClienteNombre').focus();
        return;
    }
    
    const btnGuardar = document.getElementById('btnGuardarCliente');
    const textoOriginal = btnGuardar.innerHTML;
    btnGuardar.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Guardando...';
    btnGuardar.disabled = true;
    
    try {
        const response = await fetch('/api/clientes/rapido', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                nombre,
                tipo_identificacion,
                identificacion: identificacion || null,
                telefono: telefono || null,
                email: email || null,
                direccion: direccion || null
            })
        });
        
        const data = await response.json();
        
        if (data.success) {
            const select = document.getElementById('cliente-select');
            const option = document.createElement('option');
            option.value = data.cliente.id;
            const displayText = data.cliente.identificacion ? 
                `${data.cliente.nombre} (${data.cliente.identificacion})` : 
                data.cliente.nombre;
            option.textContent = displayText;
            option.dataset.detalle = JSON.stringify(data.cliente);
            select.appendChild(option);
            select.value = data.cliente.id;
            
            actualizarDetalleCliente(data.cliente);
            mostrarNotificacion(`✅ Cliente "${data.cliente.nombre}" agregado exitosamente`, 'success');
            cerrarModalClienteFn();
        } else {
            mostrarNotificacion('❌ ' + (data.detail || 'Error al agregar cliente'), 'error');
        }
    } catch (error) {
        console.error('Error:', error);
        mostrarNotificacion('❌ Error de conexión al servidor', 'error');
    } finally {
        btnGuardar.innerHTML = textoOriginal;
        btnGuardar.disabled = false;
    }
});

// ==========================================
//   DETALLE DEL CLIENTE (MODIFICADO)
// ==========================================

const clienteSelect = document.getElementById('cliente-select');
const clienteDetalle = document.getElementById('clienteDetalle');

/**
 * Actualiza el detalle del cliente mostrando solo los campos que tienen información
 */
function actualizarDetalleCliente(cliente) {
    if (!cliente || !cliente.id) {
        clienteDetalle.style.display = 'none';
        return;
    }
    
    // Referencias a los elementos
    const nombreEl = document.getElementById('clienteDetalleNombre');
    const idEl = document.getElementById('clienteDetalleID');
    const telefonoEl = document.getElementById('clienteDetalleTelefono');
    const direccionEl = document.getElementById('clienteDetalleDireccion');
    const emailEl = document.getElementById('clienteDetalleEmail');
    const direccionRow = document.getElementById('clienteDetalleDireccionRow');
    const emailRow = document.getElementById('clienteDetalleEmailRow');
    
    // Siempre mostrar nombre, ID y teléfono
    nombreEl.textContent = cliente.nombre || '-';
    idEl.textContent = cliente.identificacion || 'No registrada';
    telefonoEl.textContent = cliente.telefono || 'No registrado';
    
    // Dirección - solo mostrar si tiene valor
    const tieneDireccion = cliente.direccion && cliente.direccion.trim() !== '';
    if (tieneDireccion) {
        direccionEl.textContent = cliente.direccion;
        direccionRow.style.display = 'block';
    } else {
        direccionRow.style.display = 'none';
    }
    
    // Email - solo mostrar si tiene valor
    const tieneEmail = cliente.email && cliente.email.trim() !== '';
    if (tieneEmail) {
        emailEl.textContent = cliente.email;
        emailRow.style.display = 'block';
    } else {
        emailRow.style.display = 'none';
    }
    
    clienteDetalle.style.display = 'block';
}

if (clienteSelect) {
    clienteSelect.addEventListener('change', function(e) {
        const selectedOption = this.options[this.selectedIndex];
        
        if (!this.value) {
            clienteDetalle.style.display = 'none';
            return;
        }
        
        if (selectedOption.dataset.detalle) {
            try {
                const detalle = JSON.parse(selectedOption.dataset.detalle);
                actualizarDetalleCliente(detalle);
                return;
            } catch (e) {
                console.warn('Error al parsear detalle del cliente:', e);
            }
        }
        
        fetch(`/api/clientes/${this.value}`)
            .then(res => {
                if (!res.ok) throw new Error('Cliente no encontrado');
                return res.json();
            })
            .then(data => {
                if (data) {
                    actualizarDetalleCliente(data);
                    selectedOption.dataset.detalle = JSON.stringify(data);
                }
            })
            .catch(error => {
                console.error('Error al cargar detalles del cliente:', error);
                clienteDetalle.style.display = 'none';
            });
    });
    
    if (clienteSelect.value) {
        clienteSelect.dispatchEvent(new Event('change'));
    }
}

// ==========================================
//   BUSCADOR RÁPIDO DE CLIENTES
// ==========================================

let timeoutBusqueda;
const busquedaCliente = document.getElementById('busquedaCliente');
const resultadosBusqueda = document.getElementById('resultadosBusqueda');

if (busquedaCliente) {
    busquedaCliente.addEventListener('input', (e) => {
        clearTimeout(timeoutBusqueda);
        const termino = e.target.value.trim();
        
        if (termino.length < 2) {
            resultadosBusqueda.style.display = 'none';
            return;
        }
        
        timeoutBusqueda = setTimeout(async () => {
            try {
                const response = await fetch(`/api/clientes/buscar?termino=${encodeURIComponent(termino)}`);
                const data = await response.json();
                
                if (data.clientes && data.clientes.length > 0) {
                    resultadosBusqueda.style.display = 'block';
                    resultadosBusqueda.innerHTML = data.clientes.map(cliente => `
                        <div class="resultado-cliente" data-id="${cliente.id}" 
                             style="padding: 10px 14px; border-bottom: 1px solid var(--hadrox-border); cursor: pointer; transition: background 0.2s;">
                            <div style="font-weight: 600; color: var(--hadrox-navy);">${cliente.nombre}</div>
                            <div style="font-size: 12px; color: var(--hadrox-light);">
                                ${cliente.identificacion ? `${cliente.tipo_identificacion || 'ID'}: ${cliente.identificacion}` : 'Sin identificación'}
                                ${cliente.telefono ? `· 📞 ${cliente.telefono}` : ''}
                            </div>
                        </div>
                    `).join('');
                    
                    resultadosBusqueda.querySelectorAll('.resultado-cliente').forEach(el => {
                        el.addEventListener('click', function() {
                            const id = parseInt(this.dataset.id);
                            const select = document.getElementById('cliente-select');
                            
                            for (let option of select.options) {
                                if (parseInt(option.value) === id) {
                                    select.value = id;
                                    break;
                                }
                            }
                            
                            busquedaCliente.value = '';
                            resultadosBusqueda.style.display = 'none';
                            select.dispatchEvent(new Event('change'));
                        });
                    });
                } else {
                    resultadosBusqueda.style.display = 'none';
                }
            } catch (error) {
                console.error('Error en búsqueda de clientes:', error);
                resultadosBusqueda.style.display = 'none';
            }
        }, 300);
    });
    
    busquedaCliente.addEventListener('blur', () => {
        setTimeout(() => {
            resultadosBusqueda.style.display = 'none';
        }, 300);
    });
}

// ==========================================
//   BOTÓN FLOTANTE DEL ESCÁNER
// ==========================================

function agregarBotonScannerFlotante() {
    const btnScanner = document.createElement('button');
    btnScanner.innerHTML = '<i class="fa-solid fa-barcode"></i> Escáner';
    btnScanner.style.cssText = `
        position: fixed;
        bottom: 20px;
        left: 20px;
        background: var(--hadrox-navy);
        color: white;
        border: none;
        padding: 12px 18px;
        border-radius: 50px;
        font-weight: 600;
        cursor: pointer;
        display: flex;
        align-items: center;
        gap: 8px;
        z-index: 100;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        transition: all 0.2s;
        font-size: 13px;
    `;
    btnScanner.onmouseover = () => btnScanner.style.transform = 'translateY(-2px)';
    btnScanner.onmouseout = () => btnScanner.style.transform = 'translateY(0)';
    btnScanner.onclick = abrirScanner;
    document.body.appendChild(btnScanner);
}

// ==========================================
//   INICIALIZACIÓN
// ==========================================

document.addEventListener('DOMContentLoaded', function() {
    const modalOverlay = document.getElementById('modal-overlay');
    if (modalOverlay) {
        modalOverlay.addEventListener('click', function(e) {
            if (e.target === modalOverlay) {
                cerrarModal();
            }
        });
    }
    
    // Verificar scanner cada 5 segundos
    setInterval(verificarScannerActivo, 5000);
    verificarScannerActivo();
    agregarBotonScannerFlotante();
});
