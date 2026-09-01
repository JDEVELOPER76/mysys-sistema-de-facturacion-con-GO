

/* Extracted from _perfil_modales.html */

    // --- Menú desplegable del usuario ---
    function pmToggleMenu() {
        document.getElementById('pmUserMenu').classList.toggle('open');
    }
    document.addEventListener('click', function (e) {
        const menu = document.getElementById('pmUserMenu');
        if (menu && !menu.contains(e.target)) menu.classList.remove('open');
    });

    function pmAbrirModal(id) {
        document.getElementById('pmUserMenu')?.classList.remove('open');
        document.getElementById(id).classList.add('open');
    }
    function pmCerrarModal(id) {
        document.getElementById(id).classList.remove('open');
    }

    function pmIniciales(nombre) {
        return nombre ? nombre.trim().charAt(0).toUpperCase() : 'A';
    }

    function pmEsc(texto) {
        var div = document.createElement('div');
        div.textContent = texto || '';
        return div.innerHTML;
    }

    function pmAvatarFallback(elemento, nombre) {
        if (!elemento) return;
        var fallback = document.createElement('span');
        fallback.id = elemento.id || '';
        fallback.className = (elemento.className || '') + ' pm-avatar-fallback';
        fallback.textContent = pmIniciales(nombre);
        elemento.replaceWith(fallback);
    }

    // --- Apartado "En línea" (widget embebido, se auto-actualiza) ---
    async function pmCargarEnLinea() {
        const cont = document.getElementById('pmListaEnLinea');
        if (!cont) return; // esta página no tiene el widget embebido

        try {
            const res = await fetch('/api/usuarios/en_linea');
            if (!res.ok) throw new Error('No se pudo obtener la lista');
            const data = await res.json();

            const contador = document.getElementById('pmOnlineCount');
            if (contador) contador.textContent = data.total;

            if (!data.conectados || data.conectados.length === 0) {
                cont.innerHTML = '<p class="pm-empty">No hay nadie más en línea en este momento.</p>';
                return;
            }

            cont.innerHTML = '<div class="pm-online-grid">' + data.conectados.map(function (u) {
                const avatar = u.foto
                    ? '<img class="pm-online-avatar" src="' + pmEsc(u.foto) + '" alt="' + pmEsc(u.username) + '" onerror="this.replaceWith(Object.assign(document.createElement(\'span\'),{className:\'pm-online-avatar\',textContent:\'' + pmIniciales(u.nombre) + '\'}))">'
                    : '<span class="pm-online-avatar">' + pmIniciales(u.nombre) + '</span>';
                return '' +
                    '<div class="pm-online-item">' +
                        avatar +
                        '<div class="pm-online-info">' +
                            '<strong>' + u.nombre + '</strong>' +
                            '<span>@' + u.username + (u.puesto ? ' · ' + u.puesto : '') + '</span>' +
                        '</div>' +
                        '<div class="pm-online-status"><span class="dot"></span> ' + u.ultima_actividad + '</div>' +
                    '</div>';
            }).join('') + '</div>';
        } catch (err) {
            cont.innerHTML = '<p class="pm-empty">Ocurrió un error al cargar los usuarios en línea.</p>';
        }
    }

    // Primera carga y luego actualización automática cada 8 segundos
    document.addEventListener('DOMContentLoaded', function () {
        pmCargarEnLinea();
        setInterval(pmCargarEnLinea, 8000);
    });

    // --- Apartado "Perfil" ---
    let pmFotoSeleccionada = null;

    async function pmAbrirPerfil() {
        pmAbrirModal('pmOverlayPerfil');
        document.getElementById('pmPerfilMsg').innerHTML = '';
        try {
            const res = await fetch('/api/perfil');
            if (!res.ok) throw new Error('No se pudo obtener el perfil');
            const data = await res.json();
            document.getElementById('pmPerfilUsername').value = data.username || '';
            document.getElementById('pmPerfilNombre').value = data.nombre || '';
            document.getElementById('pmPerfilPuesto').value = data.puesto || '';
            pmActualizarPreviewFoto(data.foto, data.nombre);
        } catch (err) {
            document.getElementById('pmPerfilMsg').innerHTML = '<p class="pm-msg err">No se pudo cargar tu perfil.</p>';
        }
    }

    function pmActualizarPreviewFoto(foto, nombre) {
        const preview = document.getElementById('pmPerfilFotoPreview');
        if (!preview) return;
        if (foto) {
            var imagen = document.createElement('img');
            imagen.id = 'pmPerfilFotoPreview';
            imagen.className = 'pm-perfil-foto-preview';
            imagen.alt = 'Foto de perfil';
            imagen.src = foto;
            imagen.addEventListener('error', function () { pmAvatarFallback(imagen, nombre); }, { once: true });
            preview.replaceWith(imagen);
        } else {
            var inicial = document.createElement('span');
            inicial.id = 'pmPerfilFotoPreview';
            inicial.className = 'pm-perfil-foto-preview';
            inicial.textContent = pmIniciales(nombre);
            preview.replaceWith(inicial);
        }
    }

    document.addEventListener('change', function (e) {
        if (e.target && e.target.id === 'pmInputFoto') {
            const file = e.target.files[0];
            if (!file) return;
            pmFotoSeleccionada = file;
            pmSubirFotoPerfil(file);
        }
    });

    async function pmSubirFotoPerfil(file) {
        const msg = document.getElementById('pmPerfilMsg');
        const formData = new FormData();
        formData.append('imagen_archivo', file);
        try {
            const res = await fetch('/api/perfil/foto', { method: 'POST', body: formData });
            const data = await res.json();
            if (!res.ok) throw new Error(data.detail || 'Error al subir la foto');
            pmActualizarPreviewFoto(data.foto, document.getElementById('pmPerfilNombre').value);
            pmActualizarAvatarTopbar(data.foto);
            msg.innerHTML = '<p class="pm-msg ok">Foto actualizada correctamente.</p>';
        } catch (err) {
            msg.innerHTML = '<p class="pm-msg err">' + err.message + '</p>';
        }
    }

    function pmActualizarAvatarTopbar(foto) {
        const wrap = document.getElementById('pmAvatarWrap');
        if (!wrap) return;
        if (!foto) return;
        wrap.innerHTML = '';
        var imagen = document.createElement('img');
        imagen.src = foto;
        imagen.alt = 'avatar';
        imagen.addEventListener('error', function () { pmAvatarFallback(imagen, document.getElementById('pmUserName')?.textContent); }, { once: true });
        wrap.appendChild(imagen);
    }

    async function pmGuardarPerfil(e) {
        e.preventDefault();
        const msg = document.getElementById('pmPerfilMsg');
        const formData = new FormData();
        formData.append('nombre', document.getElementById('pmPerfilNombre').value.trim());
        formData.append('puesto', document.getElementById('pmPerfilPuesto').value.trim());
        try {
            const res = await fetch('/api/perfil/editar', { method: 'POST', body: formData });
            const data = await res.json();
            if (!res.ok) throw new Error(data.detail || 'No se pudo guardar el perfil');
            msg.innerHTML = '<p class="pm-msg ok">Perfil actualizado correctamente.</p>';
            const nameEl = document.getElementById('pmUserName');
            if (nameEl) nameEl.textContent = document.getElementById('pmPerfilNombre').value.trim();
        } catch (err) {
            msg.innerHTML = '<p class="pm-msg err">' + err.message + '</p>';
        }
        return false;
    }
