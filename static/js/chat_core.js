/* MySYS — Núcleo de chat multi-sala (equipo general + conversaciones privadas).
   Lo usan chats.js (admin) y empleado_chat.js (empleado) mediante
   window.MySYSChatCore.iniciar({ usuario, estadoClase }).
   Expone window.MySYSChat.abrirPrivado(username, nombre, foto) para que el
   panel "Miembros del equipo" abra conversaciones 1 a 1 (salas dm:a:b que el
   servidor solo deja leer a los dos participantes). */
window.MySYSChatCore = (function () {
    'use strict';

    let cfg = {};
    let socket;
    let salaActual = 'general';
    const salasUnidas = new Set();
    const noLeidos = {};              // sala -> cantidad sin leer
    const infoUsuarios = {};          // username -> { nombre, foto }
    const protocolo = location.protocol === 'https:' ? 'wss' : 'ws';

    let mensajes, formulario, entrada, estado, titulo, subtitulo;

    // ── Utilidades ──────────────────────────────────────────────────────────
    function esc(texto) {
        const div = document.createElement('div');
        div.textContent = texto == null ? '' : String(texto);
        return div.innerHTML;
    }

    function asegurarToast() {
        let t = document.getElementById('toast');
        if (!t) {
            t = document.createElement('div');
            t.id = 'toast';
            t.className = 'toast';
            document.body.appendChild(t);
        }
        return t;
    }

    function mostrarToast(mensaje, esError) {
        const toast = asegurarToast();
        toast.textContent = mensaje;
        toast.classList.toggle('error', !!esError);
        toast.classList.add('show');
        setTimeout(() => toast.classList.remove('show'), 2800);
    }

    function estadoSocket(texto, conectado) {
        if (!estado) return;
        estado.textContent = texto;
        estado.className = cfg.estadoClase || 'chat-status';
        if (conectado) estado.classList.add('connected');
        const dot = estado.querySelector('.status-dot');
        if (dot) dot.style.background = conectado ? 'var(--hadrox-green, #22c55e)' : 'var(--hadrox-light, #94a3b8)';
    }

    function obtenerInicial(nombre) {
        if (!nombre) return '?';
        return nombre.trim().charAt(0).toUpperCase();
    }

    function formatearFechaHora(fechaStr) {
        if (!fechaStr) return { fecha: '', hora: '' };
        try {
            const fecha = new Date(fechaStr);
            if (isNaN(fecha.getTime())) return { fecha: fechaStr, hora: '' };
            const hoy = new Date();
            const esHoy = fecha.toDateString() === hoy.toDateString();
            const ayer = new Date(hoy);
            ayer.setDate(ayer.getDate() - 1);
            const esAyer = fecha.toDateString() === ayer.toDateString();
            let fechaTexto;
            if (esHoy) fechaTexto = 'Hoy';
            else if (esAyer) fechaTexto = 'Ayer';
            else fechaTexto = fecha.toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: fecha.getFullYear() !== hoy.getFullYear() ? 'numeric' : undefined });
            const hora = fecha.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' });
            return { fecha: fechaTexto, hora };
        } catch (e) {
            return { fecha: fechaStr, hora: '' };
        }
    }

    function crearAvatarHTML(usuario, foto) {
        if (foto) {
            return `<img src="${esc(foto)}" alt="${esc(usuario)}" class="chat-avatar-img" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
                    <span class="chat-avatar-initial" style="display:none;">${obtenerInicial(usuario)}</span>`;
        }
        return `<span class="chat-avatar-initial">${obtenerInicial(usuario)}</span>`;
    }

    // ── Salas ───────────────────────────────────────────────────────────────
    function salaDm(otro) {
        return 'dm:' + [cfg.usuario, otro].sort().join(':');
    }

    function otroDe(sala) {
        if (!sala || !sala.startsWith('dm:')) return '';
        const partes = sala.slice(3).split(':');
        return partes[0] === cfg.usuario ? partes[1] : partes[0];
    }

    function botonSala(sala) {
        return document.querySelector('[data-sala="' + sala + '"]');
    }

    function asegurarBadge(btn) {
        let b = btn.querySelector('.chat-room-badge');
        if (!b) {
            b = document.createElement('span');
            b.className = 'chat-room-badge';
            b.style.display = 'none';
            btn.appendChild(b);
        }
        return b;
    }

    function pintarBadge(sala) {
        const btn = botonSala(sala);
        if (!btn) return;
        const b = asegurarBadge(btn);
        const n = noLeidos[sala] || 0;
        b.textContent = n;
        b.style.display = n > 0 ? '' : 'none';
    }

    function marcarActiva(sala) {
        document.querySelectorAll('[data-sala]').forEach(b => {
            b.classList.toggle('active', b.dataset.sala === sala);
        });
    }

    function actualizarPreview(sala, contenido) {
        const btn = botonSala(sala);
        if (!btn) return;
        const prev = btn.querySelector('.dm-preview');
        if (prev) prev.textContent = contenido || '';
    }

    function crearBotonDm(sala, otro, nombre, foto) {
        const generalBtn = botonSala('general');
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = ((generalBtn ? generalBtn.className : 'chat-room') || '').replace(/\bactive\b/g, '').trim() + ' chat-room-dm';
        btn.dataset.sala = sala;
        const avatar = foto
            ? `<img class="dm-foto" src="${esc(foto)}" alt="${esc(nombre)}">`
            : `<span class="dm-avatar">${obtenerInicial(nombre)}</span>`;
        btn.innerHTML = `${avatar}
            <span class="dm-info">
                <span class="dm-nombre">${esc(nombre)}</span>
                <span class="dm-preview"></span>
            </span>`;
        btn.addEventListener('click', () => cambiarSala(sala));
        // Insertar después de "Equipo general"
        if (generalBtn && generalBtn.parentNode) {
            generalBtn.parentNode.insertBefore(btn, generalBtn.nextSibling);
        }
        return btn;
    }

    function cambiarSala(sala) {
        salaActual = sala;
        marcarActiva(sala);
        noLeidos[sala] = 0;
        pintarBadge(sala);

        if (sala === 'general') {
            if (titulo) titulo.textContent = 'Equipo general';
            if (subtitulo) subtitulo.textContent = 'Todos los administradores y empleados';
            if (entrada) entrada.placeholder = 'Escribe un mensaje…';
        } else {
            const otro = otroDe(sala);
            const info = infoUsuarios[otro] || { nombre: otro };
            if (titulo) titulo.textContent = info.nombre || otro;
            if (subtitulo) subtitulo.textContent = 'Conversación privada · solo la ven ustedes dos';
            if (entrada) entrada.placeholder = `Escribe un mensaje a ${info.nombre || otro}…`;
        }

        mensajes.innerHTML = '<div class="chat-empty"><i class="fa-regular fa-comment-dots"></i> Cargando mensajes…</div>';
        unirse(sala);
        // Si ya estábamos unidos, pedir historial de nuevo con otro join
        if (salasUnidas.has(sala) && socket && socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: 'join', sala }));
        }
        if (entrada) entrada.focus();
    }

    function unirse(sala) {
        if (salasUnidas.has(sala)) return;
        salasUnidas.add(sala);
        if (socket && socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: 'join', sala }));
        }
    }

    // ── Render de mensajes ──────────────────────────────────────────────────
    function construirMensajeHTML(m) {
        const { fecha, hora } = formatearFechaHora(m.enviado_en);
        const propio = m.usuario === cfg.usuario;
        const rolLabel = m.rol === 'admin' ? 'Admin' : 'Empleado';
        const rolClass = m.rol === 'admin' ? 'admin' : '';
        const avatarHTML = crearAvatarHTML(m.usuario, m.foto);
        return {
            fecha,
            propio,
            html: `
                <div class="chat-avatar">${avatarHTML}</div>
                <div class="chat-content">
                    <div class="chat-header">
                        <strong class="chat-name">${esc(m.usuario)}</strong>
                        <span class="chat-role ${rolClass}">${rolLabel}</span>
                    </div>
                    <div class="chat-bubble">
                        <div class="chat-text">${esc(m.contenido)}</div>
                        <div class="chat-time-badge"><i class="fa-regular fa-clock"></i> ${esc(hora)}</div>
                    </div>
                </div>
            `
        };
    }

    function dibujar(lista) {
        mensajes.innerHTML = '';
        if (!lista || lista.length === 0) {
            const esDm = salaActual !== 'general';
            mensajes.innerHTML = `<div class="chat-empty"><i class="fa-regular fa-comment-dots" style="font-size:32px;display:block;margin-bottom:12px;color:var(--hadrox-border,#cbd5e1)"></i>${esDm ? 'Sin mensajes aún. Esta conversación es privada: solo la ven ustedes dos.' : 'Todavía no hay mensajes. Sé el primero en escribir.'}</div>`;
            return;
        }
        let fechaActual = null;
        lista.forEach(m => {
            const { fecha, propio, html } = construirMensajeHTML(m);
            if (fecha && fecha !== fechaActual) {
                fechaActual = fecha;
                const separador = document.createElement('div');
                separador.className = 'chat-date-separator';
                separador.innerHTML = `<span>${esc(fecha)}</span>`;
                mensajes.appendChild(separador);
            }
            const item = document.createElement('article');
            item.className = `chat-message ${propio ? 'mine' : ''}`;
            item.innerHTML = html;
            mensajes.appendChild(item);
        });
        mensajes.scrollTop = mensajes.scrollHeight;
    }

    function agregarMensaje(m) {
        const vacio = mensajes.querySelector('.chat-empty');
        if (vacio) vacio.remove();
        const { fecha, propio, html } = construirMensajeHTML(m);
        const ultimoSeparador = mensajes.querySelector('.chat-date-separator:last-of-type span');
        if (fecha && (!ultimoSeparador || ultimoSeparador.textContent !== fecha)) {
            const separador = document.createElement('div');
            separador.className = 'chat-date-separator';
            separador.innerHTML = `<span>${esc(fecha)}</span>`;
            mensajes.appendChild(separador);
        }
        const item = document.createElement('article');
        item.className = `chat-message ${propio ? 'mine' : ''}`;
        item.innerHTML = html;
        mensajes.appendChild(item);
        mensajes.scrollTop = mensajes.scrollHeight;
        return propio;
    }

    // ── WebSocket ───────────────────────────────────────────────────────────
    function alAbrir() {
        estadoSocket('Conectado', true);
        // Unirse a general y a todas las salas presentes en la lista
        unirse('general');
        document.querySelectorAll('[data-sala]').forEach(b => unirse(b.dataset.sala));
        // salasUnidas conservadas: reenviar join tras reconexión
        salasUnidas.forEach(sala => {
            socket.send(JSON.stringify({ type: 'join', sala }));
        });
    }

    function alMensaje(e) {
        let data;
        try { data = JSON.parse(e.data); } catch (err) { return; }

        if (data.type === 'history') {
            if (data.sala === salaActual) dibujar(data.mensajes);
            return;
        }
        if (data.type === 'error') {
            mostrarToast(data.contenido || 'Error de chat', true);
            return;
        }
        if (data.type === 'message' && data.mensaje) {
            const sala = data.sala || data.mensaje.sala || 'general';
            const m = data.mensaje;
            actualizarPreview(sala, m.contenido);

            if (sala === salaActual) {
                const propio = agregarMensaje(m);
                if (!propio && document.hidden) {
                    noLeidos[sala] = (noLeidos[sala] || 0) + 1;
                    pintarBadge(sala);
                }
            } else if (m.usuario !== cfg.usuario) {
                // Mensaje de otra sala: crear botón si es un DM nuevo
                if (sala.startsWith('dm:') && !botonSala(sala)) {
                    const otro = otroDe(sala);
                    const info = infoUsuarios[otro] || {};
                    crearBotonDm(sala, otro, info.nombre || otro, info.foto || m.foto || null);
                }
                noLeidos[sala] = (noLeidos[sala] || 0) + 1;
                pintarBadge(sala);
                if (sala.startsWith('dm:')) {
                    const otro = otroDe(sala);
                    const info = infoUsuarios[otro] || {};
                    mostrarToast(`💬 ${info.nombre || otro}: ${m.contenido.substring(0, 60)}`);
                }
            }
        }
    }

    function conectar() {
        socket = new WebSocket(`${protocolo}://${location.host}/ws/chats`);
        socket.addEventListener('open', alAbrir);
        socket.addEventListener('message', alMensaje);
        socket.addEventListener('close', () => {
            estadoSocket('Desconectado · reconectando…', false);
            setTimeout(conectar, 2500);
        });
        socket.addEventListener('error', () => socket.close());
    }

    // ── API pública ─────────────────────────────────────────────────────────
    function abrirPrivado(username, nombre, foto) {
        if (!username || username === cfg.usuario) return;
        infoUsuarios[username] = { nombre: nombre || username, foto: foto || null };
        const sala = salaDm(username);
        if (!botonSala(sala)) crearBotonDm(sala, username, nombre || username, foto || null);
        cambiarSala(sala);
    }

    function iniciar(opciones) {
        cfg = opciones || {};
        mensajes = document.getElementById('chatMessages');
        formulario = document.getElementById('chatForm');
        entrada = document.getElementById('chatInput');
        estado = document.getElementById('chatStatus');
        titulo = document.getElementById('chatTitulo');
        subtitulo = document.getElementById('chatSubtitulo');
        if (!mensajes || !formulario || !entrada) return;

        // Sembrar info de usuarios desde el panel del equipo y las salas dm
        document.querySelectorAll('.chat-user[data-username]').forEach(li => {
            infoUsuarios[li.dataset.username] = {
                nombre: li.dataset.nombre || li.dataset.username,
                foto: li.dataset.foto || null
            };
        });
        document.querySelectorAll('.chat-room-dm[data-sala]').forEach(btn => {
            const otro = otroDe(btn.dataset.sala);
            if (otro && !infoUsuarios[otro]) {
                const nombreEl = btn.querySelector('.dm-nombre');
                const fotoEl = btn.querySelector('.dm-foto');
                infoUsuarios[otro] = {
                    nombre: nombreEl ? nombreEl.textContent : otro,
                    foto: fotoEl ? fotoEl.getAttribute('src') : null
                };
            }
            btn.addEventListener('click', () => cambiarSala(btn.dataset.sala));
        });

        // Botón de equipo general
        const generalBtn = botonSala('general');
        if (generalBtn) generalBtn.addEventListener('click', () => cambiarSala('general'));

        // Envío
        formulario.addEventListener('submit', e => {
            e.preventDefault();
            const contenido = entrada.value.trim();
            if (!contenido) { mostrarToast('Escribe un mensaje antes de enviar', true); return; }
            if (!socket || socket.readyState !== WebSocket.OPEN) { mostrarToast('No hay conexión con el servidor. Reintentando…', true); return; }
            socket.send(JSON.stringify({ type: 'message', sala: salaActual, contenido }));
            entrada.value = '';
            entrada.focus();
        });
        entrada.addEventListener('keydown', e => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                formulario.requestSubmit();
            }
        });

        // Al volver a la pestaña, limpiar el badge de la sala visible
        document.addEventListener('visibilitychange', () => {
            if (!document.hidden) {
                noLeidos[salaActual] = 0;
                pintarBadge(salaActual);
            }
        });

        conectar();
    }

    return { iniciar, abrirPrivado };
})();

// Punto de entrada para el panel "Miembros del equipo"
window.MySYSChat = { abrirPrivado: (u, n, f) => window.MySYSChatCore.abrirPrivado(u, n, f) };
