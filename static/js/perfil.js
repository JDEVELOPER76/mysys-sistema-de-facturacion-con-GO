/* ============================================================
   PERFIL - JavaScript para menú de usuario y modal
   ============================================================ */

// --- Menú de usuario ---
function toggleUserMenu() {
    document.getElementById('userMenu').classList.toggle('open');
}

// Cerrar menú al hacer clic fuera
document.addEventListener('click', function (e) {
    const menu = document.getElementById('userMenu');
    if (menu && !menu.contains(e.target)) {
        menu.classList.remove('open');
    }
});

// --- Modal ---
function abrirModal(id) {
    document.getElementById('userMenu')?.classList.remove('open');
    const modal = document.getElementById(id);
    modal.classList.add('open');
    document.body.style.overflow = 'hidden';
}

function cerrarModal(id) {
    const modal = document.getElementById(id);
    modal.classList.remove('open');
    document.body.style.overflow = '';
}

// Cerrar modal con ESC
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        document.querySelectorAll('.modal-overlay.open').forEach(function(modal) {
            modal.classList.remove('open');
            document.body.style.overflow = '';
        });
    }
});

// Cerrar modal al hacer clic en el overlay
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('modal-overlay')) {
        e.target.classList.remove('open');
        document.body.style.overflow = '';
    }
});

// --- Abrir Perfil ---
async function abrirPerfil() {
    abrirModal('perfilModal');
    const msg = document.getElementById('perfilMsg');
    msg.innerHTML = '';
    
    try {
        const res = await fetch('/api/perfil');
        if (!res.ok) throw new Error('No se pudo obtener el perfil');
        const data = await res.json();
        
        document.getElementById('perfilUsername').value = data.username || '';
        document.getElementById('perfilNombre').value = data.nombre || '';
        document.getElementById('perfilPuesto').value = data.puesto || '';
        
        actualizarPreviewFoto(data.foto, data.nombre);
    } catch (err) {
        msg.innerHTML = '<p class="perfil-msg error">No se pudo cargar tu perfil.</p>';
    }
}

// --- Actualizar preview de foto ---
function actualizarPreviewFoto(foto, nombre) {
    const preview = document.getElementById('fotoPreview');
    if (!preview) return;
    
    if (foto) {
        preview.innerHTML = '<img src="' + foto + '" alt="Foto de perfil">';
    } else {
        preview.textContent = (nombre || 'A').charAt(0).toUpperCase();
    }
}

// --- Subir foto ---
document.addEventListener('change', function (e) {
    if (e.target && e.target.id === 'inputFoto') {
        const file = e.target.files[0];
        if (!file) return;
        subirFotoPerfil(file);
    }
});

async function subirFotoPerfil(file) {
    const msg = document.getElementById('perfilMsg');
    const formData = new FormData();
    formData.append('imagen_archivo', file);
    
    try {
        const res = await fetch('/api/perfil/foto', { 
            method: 'POST', 
            body: formData 
        });
        const data = await res.json();
        
        if (!res.ok) throw new Error(data.detail || 'Error al subir la foto');
        
        // Actualizar preview
        actualizarPreviewFoto(data.foto, document.getElementById('perfilNombre').value);
        
        // Actualizar avatar en el topbar
        actualizarAvatarTopbar(data.foto);
        
        msg.innerHTML = '<p class="perfil-msg success">Foto actualizada correctamente.</p>';
    } catch (err) {
        msg.innerHTML = '<p class="perfil-msg error">' + err.message + '</p>';
    }
}

// --- Actualizar avatar en topbar ---
function actualizarAvatarTopbar(foto) {
    const wrap = document.getElementById('avatarWrap');
    if (!wrap) return;
    
    if (foto) {
        wrap.innerHTML = '<img src="' + foto + '" alt="avatar" class="avatar-img">';
    } else {
        const nombre = document.getElementById('perfilNombre')?.value || '';
        wrap.innerHTML = '<span class="avatar-text">' + (nombre.charAt(0).toUpperCase() || 'A') + '</span>';
    }
}

// --- Guardar perfil ---
async function guardarPerfil(e) {
    e.preventDefault();
    const msg = document.getElementById('perfilMsg');
    const formData = new FormData();
    
    formData.append('nombre', document.getElementById('perfilNombre').value.trim());
    formData.append('puesto', document.getElementById('perfilPuesto').value.trim());
    
    try {
        const res = await fetch('/api/perfil/editar', { 
            method: 'POST', 
            body: formData 
        });
        const data = await res.json();
        
        if (!res.ok) throw new Error(data.detail || 'No se pudo guardar el perfil');
        
        msg.innerHTML = '<p class="perfil-msg success">Perfil actualizado correctamente.</p>';
        
        // Actualizar nombre en topbar
        const nameEl = document.getElementById('userName');
        if (nameEl) {
            nameEl.textContent = document.getElementById('perfilNombre').value.trim();
        }
        
    } catch (err) {
        msg.innerHTML = '<p class="perfil-msg error">' + err.message + '</p>';
    }
    return false;
}

// --- Inicialización ---
document.addEventListener('DOMContentLoaded', function() {
    console.log('Perfil.js cargado correctamente');
});