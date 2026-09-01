/* MySYS — Chat del empleado. La lógica del chat vive en chat_core.js;
   aquí se configura la página y se conserva el estado del escáner. */
(function () {
    'use strict';

    window.MySYSChatCore.iniciar({
        usuario: document.body.dataset.usuario || '',
        estadoClase: 'chat-status'
    });

    // ── Estado del escáner (compartido) ─────────────────────────────────────
    async function verificarEstadoEscaner() {
        try {
            const resp = await fetch('/api/verificar_lecturas');
            const data = await resp.json();
            const navStatus = document.getElementById('scannerStatus');
            const navStatusText = document.getElementById('scannerStatusText');
            if (data.conectado) {
                if (navStatus) navStatus.classList.add('connected');
                if (navStatusText) navStatusText.textContent = 'Escáner conectado';
            } else {
                if (navStatus) navStatus.classList.remove('connected');
                if (navStatusText) navStatusText.textContent = 'Escáner desconectado';
            }
        } catch (err) {
            console.warn('No se pudo verificar el escáner');
        }
    }

    setInterval(verificarEstadoEscaner, 10000);
    verificarEstadoEscaner();
})();
