

/* Extracted from admin_clientes.html */

        // ============================================================
        // MENÚ DE USUARIO
        // ============================================================
        function pmToggleMenu() {
            const dropdown = document.getElementById('pmDropdown');
            const trigger = document.querySelector('.pm-user-trigger');
            const isOpen = dropdown.classList.toggle('open');
            trigger.setAttribute('aria-expanded', isOpen);
        }

        document.addEventListener('click', function(e) {
            const menu = document.getElementById('pmUserMenu');
            if (!menu.contains(e.target)) {
                const dropdown = document.getElementById('pmDropdown');
                dropdown.classList.remove('open');
                document.querySelector('.pm-user-trigger').setAttribute('aria-expanded', 'false');
            }
        });

        function pmAbrirPerfil() {
            alert('Abrir perfil de usuario');
        }

        // ============================================================
        // MODAL DE REGISTRO
        // ============================================================
        function openRegisterModal() {
            const modal = document.getElementById('registerModal');
            modal.classList.add('active');
            document.body.style.overflow = 'hidden';
            document.getElementById('registerForm').reset();
        }

        function closeRegisterModal() {
            const modal = document.getElementById('registerModal');
            modal.classList.remove('active');
            document.body.style.overflow = '';
        }

        document.getElementById('registerModal').addEventListener('click', function(e) {
            if (e.target === this) {
                closeRegisterModal();
            }
        });

        // ============================================================
        // FILTRO DE TABLA
        // ============================================================
        function filterTable() {
            const searchInput = document.getElementById('searchInput');
            const filterType = document.getElementById('filterType');
            const searchTerm = searchInput.value.toLowerCase().trim();
            const typeFilter = filterType.value;

            const rows = document.querySelectorAll('#clientTableBody tr');
            let visibleCount = 0;

            rows.forEach(row => {
                const nombre = row.getAttribute('data-nombre') || '';
                const telefono = row.getAttribute('data-telefono') || '';
                const email = row.getAttribute('data-email') || '';
                const id = row.getAttribute('data-client-id') || '';
                const tipo = row.getAttribute('data-tipo') || 'persona';

                let matchesSearch = true;
                if (searchTerm) {
                    matchesSearch = nombre.includes(searchTerm) || 
                                   telefono.includes(searchTerm) || 
                                   email.includes(searchTerm) || 
                                   id.includes(searchTerm);
                }

                let matchesType = true;
                if (typeFilter !== 'all') {
                    matchesType = tipo === typeFilter;
                }

                if (matchesSearch && matchesType) {
                    row.style.display = '';
                    visibleCount++;
                } else {
                    row.style.display = 'none';
                }
            });

            const countEl = document.getElementById('clientCount');
            if (countEl) {
                countEl.textContent = visibleCount;
            }

            const tbody = document.getElementById('clientTableBody');
            let noResultsMsg = tbody.querySelector('.no-results-msg');
            if (visibleCount === 0 && rows.length > 0) {
                if (!noResultsMsg) {
                    noResultsMsg = document.createElement('tr');
                    noResultsMsg.className = 'no-results-msg';
                    noResultsMsg.innerHTML = `
                        <td colspan="6">
                            <div class="empty-state">
                                <i class="fa-solid fa-search" style="font-size: 28px;"></i>
                                <p style="font-weight: 500; margin-top: 0.5rem;">No se encontraron clientes que coincidan con la búsqueda.</p>
                            </div>
                        </td>
                    `;
                    tbody.appendChild(noResultsMsg);
                }
                noResultsMsg.style.display = '';
            } else if (noResultsMsg) {
                noResultsMsg.style.display = 'none';
            }
        }

        function clearFilters() {
            document.getElementById('searchInput').value = '';
            document.getElementById('filterType').value = 'all';
            filterTable();
        }

        // ============================================================
        // MODAL DE DETALLE DEL CLIENTE
        // ============================================================
        const clientData = {
            {% for c in clientes %}
            {{ c.id }}: {
                id: {{ c.id }},
                nombre: "{{ c.nombre}}",
                tipo_identificacion: "{{ c.tipo_identificacion or 'No especificado' }}",
                identificacion: "{{ c.identificacion or 'No registrada' }}",
                direccion: "{{ c.direccion or 'No registrada' }}",
                telefono: "{{ c.telefono or 'No registrado' }}",
                email: "{{ c.email or 'No registrado' }}",
                fecha_registro: "{{ c.fecha_registro or 'No disponible' }}"
            },
            {% endfor %}
        };

        function openClientDetail(clientId) {
            const modal = document.getElementById('clientDetailModal');
            const grid = document.getElementById('clientDetailGrid');
            
            const data = clientData[clientId];
            if (!data) {
                grid.innerHTML = '<div class="empty-state"><p>No se encontró información del cliente.</p></div>';
                modal.classList.add('active');
                document.body.style.overflow = 'hidden';
                return;
            }

            grid.innerHTML = `
                <div class="client-detail-item full-width">
                    <div class="label">Nombre / Razón Social</div>
                    <div class="value" style="font-size: 1.1rem; color: var(--primary);">${data.nombre}</div>
                </div>
                <div class="client-detail-item">
                    <div class="label">Código de Cliente</div>
                    <div class="value"><span style="font-family: monospace; font-weight: 700; color: var(--primary);">#00${data.id}</span></div>
                </div>
                <div class="client-detail-item">
                    <div class="label">Fecha de Registro</div>
                    <div class="value">${data.fecha_registro}</div>
                </div>
                <div class="client-detail-item">
                    <div class="label">Tipo de Identificación</div>
                    <div class="value"><span class="badge ${data.tipo_identificacion === 'No especificado' ? 'badge-warning' : 'badge-success'}">${data.tipo_identificacion}</span></div>
                </div>
                <div class="client-detail-item">
                    <div class="label">Número de Identificación</div>
                    <div class="value"><span style="font-family: monospace;">${data.identificacion}</span></div>
                </div>
                <div class="client-detail-item">
                    <div class="label">Teléfono</div>
                    <div class="value"><i class="fa-solid fa-phone" style="color: var(--text-muted); margin-right: 0.4rem;"></i>${data.telefono}</div>
                </div>
                <div class="client-detail-item">
                    <div class="label">Email</div>
                    <div class="value"><i class="fa-solid fa-envelope" style="color: var(--text-muted); margin-right: 0.4rem;"></i>${data.email}</div>
                </div>
                <div class="client-detail-item full-width">
                    <div class="label">Dirección</div>
                    <div class="value"><i class="fa-solid fa-location-dot" style="color: var(--text-muted); margin-right: 0.4rem;"></i>${data.direccion}</div>
                </div>
                <div class="client-detail-item full-width" style="margin-top: 0.5rem;">
                    <div class="client-stats">
                        <div class="client-stat">
                            <div class="number">0</div>
                            <div class="stat-label">Compras realizadas</div>
                        </div>
                        <div class="client-stat">
                            <div class="number">$0.00</div>
                            <div class="stat-label">Total gastado</div>
                        </div>
                        <div class="client-stat">
                            <div class="number">0</div>
                            <div class="stat-label">Últimos 30 días</div>
                        </div>
                    </div>
                </div>
            `;

            modal.classList.add('active');
            document.body.style.overflow = 'hidden';
        }

        function closeClientDetail() {
            const modal = document.getElementById('clientDetailModal');
            modal.classList.remove('active');
            document.body.style.overflow = '';
        }

        document.getElementById('clientDetailModal').addEventListener('click', function(e) {
            if (e.target === this) {
                closeClientDetail();
            }
        });

        // Cerrar modales con Escape
        document.addEventListener('keydown', function(e) {
            if (e.key === 'Escape') {
                closeRegisterModal();
                closeClientDetail();
            }
        });

        // ============================================================
        // INICIALIZACIÓN
        // ============================================================
        document.addEventListener('DOMContentLoaded', function() {
            filterTable();
        });
    
