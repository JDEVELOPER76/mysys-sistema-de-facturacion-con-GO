let socketPanelEnLinea = null;
let cerrarPorLogout = false;

document.addEventListener('DOMContentLoaded', () => {
    conectarPanelEnLinea();
    configurarCierrePorLogout();
});

function conectarPanelEnLinea() {
    if (cerrarPorLogout) {
        return;
    }

    if (socketPanelEnLinea && socketPanelEnLinea.readyState === WebSocket.OPEN) {
        return;
    }

    const protocolo = location.protocol === 'https:' ? 'wss' : 'ws';
    socketPanelEnLinea = new WebSocket(`${protocolo}://${location.host}/ws/en_linea`);

    socketPanelEnLinea.addEventListener('message', (event) => {
        try {
            const data = JSON.parse(event.data);
            pintarUsuariosEnLinea(data);
        } catch (err) {
            console.error('Error leyendo datos de en línea:', err);
        }
    });

    socketPanelEnLinea.addEventListener('close', () => {
        socketPanelEnLinea = null;
        if (cerrarPorLogout) {
            return;
        }
        setTimeout(conectarPanelEnLinea, 3000);
    });
}

function cerrarSocketPanelEnLinea() {
    cerrarPorLogout = true;
    if (socketPanelEnLinea && socketPanelEnLinea.readyState === WebSocket.OPEN) {
        socketPanelEnLinea.close(1000, 'Logout de administrador');
    }
    socketPanelEnLinea = null;
}

function configurarCierrePorLogout() {
    document.querySelectorAll('a[href="/logout"]').forEach((enlace) => {
        enlace.addEventListener('click', cerrarSocketPanelEnLinea, { capture: true });
    });

    window.addEventListener('pagehide', cerrarSocketPanelEnLinea);
    window.addEventListener('beforeunload', cerrarSocketPanelEnLinea);
}

function pintarUsuariosEnLinea(data) {
    const contador = document.getElementById('totalEnLinea');
    const cuerpoTabla = document.getElementById('cuerpoTablaEnLinea');
    const vacioMsg = document.getElementById('enLineaVacio');

    if (contador) contador.textContent = data.total ?? 0;
    if (!cuerpoTabla) return;

    const conectados = data.conectados || [];

    if (conectados.length === 0) {
        cuerpoTabla.innerHTML = '';
        if (vacioMsg) vacioMsg.style.display = 'block';
        return;
    }
    if (vacioMsg) vacioMsg.style.display = 'none';

    cuerpoTabla.innerHTML = conectados.map(u => {
        // Función para obtener la inicial
        const inicial = u.nombre ? u.nombre.charAt(0).toUpperCase() : 'A';
        
        // Generar el avatar HTML
        let avatarHtml;
        if (u.foto) {
            avatarHtml = `<img src="${u.foto}" alt="${u.nombre}">`;
        } else {
            avatarHtml = `<span class="avatar-inicial">${inicial}</span>`;
        }

        return `
            <tr>
                <td>
                    <div class="item-meta">
                        <div class="en-linea-avatar">
                            ${avatarHtml}
                        </div>
                        <div class="item-info">
                            <strong>${u.nombre || 'Usuario'}</strong>
                            <span>@${u.username || 'usuario'}</span>
                        </div>
                    </div>
                </td>
                <td>${u.puesto || '-'}</td>
                <td>
                    <span class="badge ${u.tipo === 'admin' ? 'badge-admin' : 'badge-user'}">
                        ${u.tipo === 'admin' ? 'Administrador' : 'Operario'}
                    </span>
                </td>
                <td>
                    <span class="en-linea-dot ${u.en_vivo ? 'vivo' : 'reciente'}"></span>
                    ${u.en_vivo ? 'En línea' : 'Actividad reciente'}
                </td>
                <td>${u.ultima_actividad || '-'}</td>
            </tr>
        `;
    }).join('');
}