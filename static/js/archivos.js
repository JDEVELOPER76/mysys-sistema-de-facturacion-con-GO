(function () {
    const protocolo = location.protocol === 'https:' ? 'wss' : 'ws';
    const estadoEl = document.getElementById('estadoArchivos');
    const tablaBody = document.getElementById('tablaArchivosBody');
    const totalArchivosEl = document.getElementById('totalArchivos');
    const totalPesoEl = document.getElementById('totalPeso');
    const totalUsuariosEl = document.getElementById('totalUsuarios');
    const uploadZone = document.getElementById('uploadZone');
    const fileInput = document.getElementById('fileInput');
    const progressWrapper = document.getElementById('progressWrapper');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');
    const descripcionInput = document.getElementById('descripcionArchivo');
    const toastContainer = document.getElementById('toastContainer');

    let socket;
    let usuariosUnicos = new Set();

    function pintarEstado(texto, conectado) {
        if (!estadoEl) return;
        estadoEl.textContent = texto;
        estadoEl.classList.toggle('connected', !!conectado);
    }

    function formatoBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    function iconoPorMime(mime, nombre) {
        if (!mime) mime = '';
        const ext = (nombre || '').split('.').pop().toLowerCase();
        if (mime.includes('pdf') || ext === 'pdf') return { cls: 'pdf', icon: 'fa-file-pdf' };
        if (mime.includes('word') || /docx?/.test(ext)) return { cls: 'doc', icon: 'fa-file-word' };
        if (mime.includes('excel') || /xlsx?/.test(ext)) return { cls: 'xls', icon: 'fa-file-excel' };
        if (mime.includes('image') || /png|jpe?g|gif|webp|bmp/.test(ext)) return { cls: 'img', icon: 'fa-file-image' };
        if (mime.includes('zip') || /zip|rar|7z|tar/.test(ext)) return { cls: 'zip', icon: 'fa-file-zipper' };
        return { cls: 'gen', icon: 'fa-file' };
    }

    function avatarIniciales(nombre) {
        return (nombre || '??').split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2);
    }

    function mostrarToast(mensaje, tipo = 'info') {
        if (!toastContainer) return;
        const toast = document.createElement('div');
        toast.className = `toast ${tipo}`;
        const icono = tipo === 'success' ? 'fa-check-circle' : tipo === 'error' ? 'fa-circle-xmark' : 'fa-circle-info';
        toast.innerHTML = `<i class="fa-solid ${icono}" style="font-size:1.1rem;color:var(--${tipo === 'info' ? 'primary' : tipo},#4f46e5)"></i><span>${mensaje}</span>`;
        toastContainer.appendChild(toast);
        setTimeout(() => {
            toast.classList.add('fade-out');
            setTimeout(() => toast.remove(), 350);
        }, 3500);
    }

    function actualizarMetricas() {
        const filas = tablaBody ? tablaBody.querySelectorAll('tr') : [];
        let pesoTotal = 0;
        usuariosUnicos.clear();
        filas.forEach(tr => {
            const peso = parseInt(tr.dataset.peso || '0');
            const user = tr.dataset.uploader || '';
            pesoTotal += peso;
            if (user) usuariosUnicos.add(user);
        });
        if (totalArchivosEl) totalArchivosEl.textContent = filas.length.toLocaleString('es-ES');
        if (totalPesoEl) totalPesoEl.textContent = formatoBytes(pesoTotal);
        if (totalUsuariosEl) totalUsuariosEl.textContent = usuariosUnicos.size.toLocaleString('es-ES');
    }

    function crearFilaHTML(archivo) {
        const icono = iconoPorMime(archivo.mime_type, archivo.nombre_original);
        return `
        <tr class="archivo-row" data-id="${archivo.id}" data-peso="${archivo.tamaño_bytes}" data-uploader="${archivo.subido_por}">
            <td>
                <div class="archivo-meta">
                    <div class="archivo-icon ${icono.cls}"><i class="fa-solid ${icono.icon}"></i></div>
                    <div class="archivo-info">
                        <strong>${escapeHtml(archivo.nombre_original)}</strong>
                        <span>${archivo.descripcion ? escapeHtml(archivo.descripcion) : 'Sin descripción'}</span>
                    </div>
                </div>
            </td>
            <td class="archivo-peso">${formatoBytes(archivo.tamaño_bytes)}</td>
            <td>
                <div class="archivo-uploader">
                    <div class="avatar-mini">${avatarIniciales(archivo.subido_por)}</div>
                    <span>${escapeHtml(archivo.subido_por)}</span>
                </div>
            </td>
            <td class="archivo-fecha">${archivo.fecha_subida || ''}</td>
            <td class="actions-cell">
                <a href="/admin/archivos/descargar/${archivo.id}" class="btn-download" title="Descargar">
                    <i class="fa-solid fa-download"></i> Descargar
                </a>
                <button class="btn-delete-file" onclick="eliminarArchivo(${archivo.id})" title="Eliminar">
                    <i class="fa-solid fa-trash"></i>
                </button>
            </td>
        </tr>`;
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function agregarArchivoALaTabla(archivo, alInicio = true) {
        if (!tablaBody) return;
        const html = crearFilaHTML(archivo);
        if (alInicio) {
            tablaBody.insertAdjacentHTML('afterbegin', html);
        } else {
            tablaBody.insertAdjacentHTML('beforeend', html);
        }
        actualizarMetricas();
    }

    function removerArchivoDeTabla(id) {
        if (!tablaBody) return;
        const fila = tablaBody.querySelector(`tr[data-id="${id}"]`);
        if (fila) {
            fila.remove();
            actualizarMetricas();
        }
    }

    function conectarWS() {
        socket = new WebSocket(`${protocolo}://${location.host}/ws/archivos`);
        socket.addEventListener('open', () => {
            pintarEstado('En vivo · actualizando automáticamente', true);
        });
        socket.addEventListener('message', (event) => {
            try {
                const data = JSON.parse(event.data);
                if (data.type === 'nuevo_archivo') {
                    agregarArchivoALaTabla(data.archivo);
                    mostrarToast(`📁 ${escapeHtml(data.archivo.nombre_original)} subido por ${escapeHtml(data.archivo.subido_por)}`, 'info');
                } else if (data.type === 'archivo_eliminado') {
                    removerArchivoDeTabla(data.id);
                    mostrarToast('Archivo eliminado', 'info');
                }
            } catch (e) { console.error('WS error:', e); }
        });
        socket.addEventListener('close', () => {
            pintarEstado('Desconectado · reconectando…', false);
            setTimeout(conectarWS, 3000);
        });
        socket.addEventListener('error', () => { if (socket) socket.close(); });
    }

    function subirArchivo(file) {
        if (!file) return;
        const formData = new FormData();
        formData.append('archivo', file);
        if (descripcionInput) formData.append('descripcion', descripcionInput.value.trim());

        if (progressWrapper) progressWrapper.classList.add('active');
        if (progressFill) progressFill.style.width = '0%';
        if (progressText) progressText.textContent = '0%';

        const xhr = new XMLHttpRequest();
        xhr.open('POST', '/admin/archivos/subir', true);

        xhr.upload.addEventListener('progress', (e) => {
            if (e.lengthComputable) {
                const pct = Math.round((e.loaded / e.total) * 100);
                if (progressFill) progressFill.style.width = pct + '%';
                if (progressText) progressText.textContent = pct + '%';
            }
        });

        xhr.addEventListener('load', () => {
            if (progressWrapper) progressWrapper.classList.remove('active');
            if (progressFill) progressFill.style.width = '0%';
            if (descripcionInput) descripcionInput.value = '';

            if (xhr.status === 200) {
                try {
                    const res = JSON.parse(xhr.responseText);
                    if (res.success) {
                        mostrarToast('Archivo subido correctamente', 'success');
                    } else {
                        mostrarToast(res.error || 'Error al subir', 'error');
                    }
                } catch { mostrarToast('Archivo subido', 'success'); }
            } else {
                mostrarToast('Error al subir el archivo', 'error');
            }
        });

        xhr.addEventListener('error', () => {
            if (progressWrapper) progressWrapper.classList.remove('active');
            mostrarToast('Error de red al subir', 'error');
        });

        xhr.send(formData);
    }

    if (uploadZone && fileInput) {
        uploadZone.addEventListener('click', () => fileInput.click());
        uploadZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            uploadZone.classList.add('dragover');
        });
        uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('dragover'));
        uploadZone.addEventListener('drop', (e) => {
            e.preventDefault();
            uploadZone.classList.remove('dragover');
            const files = e.dataTransfer.files;
            if (files.length) subirArchivo(files[0]);
        });
        fileInput.addEventListener('change', () => {
            if (fileInput.files.length) subirArchivo(fileInput.files[0]);
        });
    }

    window.eliminarArchivo = async function (id) {
        if (!confirm('¿Eliminar este archivo permanentemente?')) return;
        try {
            const res = await fetch(`/admin/archivos/eliminar/${id}`, { method: 'POST' });
            if (res.ok) {
                mostrarToast('Archivo eliminado', 'success');
            } else {
                mostrarToast('Error al eliminar', 'error');
            }
        } catch {
            mostrarToast('Error de red', 'error');
        }
    };

    actualizarMetricas();
    conectarWS();
})();