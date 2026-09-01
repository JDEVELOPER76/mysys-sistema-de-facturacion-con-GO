

/* Extracted from register.html */

        // Pequeño script para mostrar/ocultar contraseña
        function togglePass() {
            const passInput = document.getElementById('password');
            const toggleBtn = document.querySelector('.toggle-password');
            
            if (passInput.type === 'password') {
                passInput.type = 'text';
                toggleBtn.textContent = 'Ocultar';
            } else {
                passInput.type = 'password';
                toggleBtn.textContent = 'Mostrar';
            }
        }
    
