/* MySYS — Chat del administrador. La lógica vive en chat_core.js;
   aquí solo se configura la página. */
(function () {
    'use strict';
    window.MySYSChatCore.iniciar({
        usuario: document.body.dataset.usuario || '',
        estadoClase: 'chat-live'
    });
})();
