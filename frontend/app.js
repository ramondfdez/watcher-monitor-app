// Configuración de la API
const API_URL = window.location.hostname === 'localhost' 
    ? 'http://localhost:8080/api'
    : `http://${window.location.hostname}:8080/api`;

let containers = [];
let currentTab = 'dashboard';
let cpuChart, memoryChart;
let cpuData = [];
let memoryData = [];
const MAX_DATA_POINTS = 20;

// Inicialización
document.addEventListener('DOMContentLoaded', () => {
    initCharts();
    switchTab('dashboard');
    
    // Auto-refresh cada 5 segundos
    setInterval(() => {
        if (currentTab === 'dashboard') {
            loadSystemStats();
        } else if (currentTab === 'containers') {
            loadContainers();
        } else if (currentTab === 'processes') {
            loadProcesses();
        }
    }, 5000);
});

// Cambiar de pestaña
function switchTab(tabName) {
    currentTab = tabName;
    // Actualizar navegación
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.remove('active');
    });
    document.querySelector(`[data-tab="${tabName}"]`).classList.add('active');
    
    // Actualizar contenido
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    document.getElementById(`${tabName}-tab`).classList.add('active');
    
    // Actualizar título
    const titles = {
        'dashboard': 'Dashboard del Sistema',
        'containers': 'Gestión de Contenedores',
        'processes': 'Procesos y Sistema'
    };
    const subtitles = {
        'dashboard': 'Monitoreo en tiempo real',
        'containers': 'Control y administración',
        'processes': 'Gestión de procesos y acciones del sistema'
    };
    
    document.getElementById('page-title').textContent = titles[tabName];
    document.getElementById('page-subtitle').textContent = subtitles[tabName];
    
    // Cargar datos
    if (tabName === 'dashboard') {
        loadSystemStats();
    } else if (tabName === 'containers') {
        loadContainers();
    } else if (tabName === 'processes') {
        loadProcesses();
        setupSystemActions();
    }
}

// Inicializar gráficas
function initCharts() {
    const commonOptions = {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
            legend: {
                display: false
            }
        },
        scales: {
            y: {
                beginAtZero: true,
                max: 100,
                ticks: {
                    callback: function(value) {
                        return value + '%';
                    }
                }
            },
            x: {
                display: false
            }
        },
        animation: {
            duration: 750
        }
    };

    // CPU Chart
    const cpuCtx = document.getElementById('cpuChart').getContext('2d');
    cpuChart = new Chart(cpuCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'CPU',
                data: [],
                borderColor: '#667eea',
                backgroundColor: 'rgba(102, 126, 234, 0.1)',
                borderWidth: 3,
                fill: true,
                tension: 0.4,
                pointRadius: 0
            }]
        },
        options: commonOptions
    });

    // Memory Chart
    const memoryCtx = document.getElementById('memoryChart').getContext('2d');
    memoryChart = new Chart(memoryCtx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'Memoria',
                data: [],
                borderColor: '#f5576c',
                backgroundColor: 'rgba(245, 87, 108, 0.1)',
                borderWidth: 3,
                fill: true,
                tension: 0.4,
                pointRadius: 0
            }]
        },
        options: commonOptions
    });
}

// Cargar estadísticas del sistema
async function loadSystemStats() {
    try {
        const response = await fetch(`${API_URL}/stats`);
        
        if (!response.ok) {
            throw new Error(`Error HTTP: ${response.status}`);
        }
        
        const stats = await response.json();
        updateDashboard(stats);
    } catch (error) {
        console.error('Error al cargar estadísticas:', error);
    }
}

// Actualizar dashboard
function updateDashboard(stats) {
    // System Info Banner
    document.getElementById('hostname').textContent = stats.hostname || 'unknown';
    document.getElementById('uptime').textContent = stats.uptime || 'N/A';
    document.getElementById('local-ip').textContent = stats.local_ip || '127.0.0.1';
    
    const temp = stats.temperature || 'N/A';
    document.getElementById('temperature').textContent = temp !== 'N/A' ? temp + '°C' : 'N/A';
    
    // CPU
    const cpuUsage = parseFloat(stats.cpu_usage) || 0;
    document.getElementById('cpu-value').textContent = cpuUsage.toFixed(1) + '%';
    
    const loadAvg = stats.load_average || '0.0';
    document.getElementById('cpu-load').textContent = `Load: ${loadAvg}`;
    
    // Memoria
    const memoryUsed = parseFloat(stats.memory_used_mb) || 0;
    const memoryTotal = parseFloat(stats.memory_total_mb) || 1024;
    const memoryAvailable = parseFloat(stats.memory_available_mb) || 0;
    const memoryPercent = (memoryUsed / memoryTotal) * 100;
    
    document.getElementById('memory-value').textContent = memoryUsed.toFixed(0) + ' MB';
    document.getElementById('memory-total').textContent = `de ${memoryTotal.toFixed(0)} MB (${memoryPercent.toFixed(1)}%) · ${memoryAvailable.toFixed(0)} MB libre`;
    
    // Disco
    const diskUsage = parseFloat(stats.disk_usage) || 0;
    document.getElementById('disk-value').textContent = diskUsage.toFixed(0) + '%';
    document.getElementById('disk-space').textContent = stats.disk_space || '0/0';
    
    // Red
    document.getElementById('network-interface').textContent = stats.network_interface || '-';
    
    // Velocidad de red
    const downloadSpeed = parseFloat(stats.download_speed) || 0;
    const uploadSpeed = parseFloat(stats.upload_speed) || 0;
    document.getElementById('network-speed').textContent = `↓ ${downloadSpeed.toFixed(2)} Mbps ↑ ${uploadSpeed.toFixed(2)} Mbps`;
    
    // Contenedores
    const containersRunning = stats.containers_running || 0;
    const containersTotal = stats.containers_total || 0;
    
    document.getElementById('containers-running').textContent = containersRunning;
    document.getElementById('containers-total').textContent = `de ${containersTotal} total`;
    
    // Imágenes
    const imagesCount = stats.images_count || 0;
    document.getElementById('images-count').textContent = imagesCount;
    
    // Actualizar gráficas
    updateChart(cpuChart, cpuData, cpuUsage);
    updateChart(memoryChart, memoryData, memoryPercent);
}

// Actualizar gráfica
function updateChart(chart, dataArray, newValue) {
    dataArray.push(newValue);
    
    if (dataArray.length > MAX_DATA_POINTS) {
        dataArray.shift();
    }
    
    chart.data.labels = dataArray.map((_, i) => i);
    chart.data.datasets[0].data = dataArray;
    chart.update('none');
}

// Cargar contenedores
async function loadContainers() {
    const loadingEl = document.getElementById('loading');
    const errorEl = document.getElementById('error');
    
    loadingEl.style.display = 'block';
    errorEl.style.display = 'none';

    try {
        const response = await fetch(`${API_URL}/containers`);
        
        if (!response.ok) {
            throw new Error(`Error HTTP: ${response.status}`);
        }
        
        containers = await response.json();
        renderContainers();
        loadingEl.style.display = 'none';
    } catch (error) {
        console.error('Error al cargar contenedores:', error);
        errorEl.textContent = `Error al conectar con el servidor: ${error.message}`;
        errorEl.style.display = 'block';
        loadingEl.style.display = 'none';
    }
}

// Renderizar contenedores
function renderContainers() {
    const containersList = document.getElementById('containersList');
    
    if (!containers || containers.length === 0) {
        containersList.innerHTML = '<div class="loading-state"><p>No hay contenedores disponibles</p></div>';
        return;
    }

    // Separar contenedores propios de la app vs otros
    const appContainerNames = ['docker-manager-backend', 'docker-manager-frontend'];
    const otherContainers = [];
    const appContainers = [];
    
    containers.forEach(container => {
        const containerName = (container.name || '').replace(/^\//, ''); // Remover / inicial si existe
        if (appContainerNames.includes(containerName)) {
            appContainers.push(container);
        } else {
            otherContainers.push(container);
        }
    });
    
    // Renderizar primero otros contenedores, luego los de la app
    const allContainersHTML = [...otherContainers, ...appContainers].map(container => {
        const containerName = (container.name || '').replace(/^\//, '');
        const isAppContainer = appContainerNames.includes(containerName);
        const cardClass = isAppContainer ? 'container-card-app' : '';
        
        return `
        <div class="container-card ${container.state ? container.state.toLowerCase() : 'unknown'} ${cardClass}">
            <div class="container-header">
                <div class="container-name">
                    ${container.name || 'Sin nombre'}
                    ${isAppContainer ? '<span class="app-badge">Sistema</span>' : ''}
                </div>
                <span class="status-badge">${container.state || 'unknown'}</span>
            </div>
            
            <div class="container-info">
                <div>
                    <strong>ID:</strong>
                    <span class="container-id">${container.id ? container.id.substring(0, 12) : 'N/A'}</span>
                </div>
                <div>
                    <strong>Imagen:</strong>
                    <span>${container.image || 'N/A'}</span>
                </div>
                <div>
                    <strong>Estado:</strong>
                    <span>${container.status || 'N/A'}</span>
                </div>
            </div>

            <div class="container-actions">
                ${!isAppContainer ? `
                    ${container.state === 'running' ? `
                        <button class="btn btn-warning btn-sm" onclick="stopContainer('${container.id}', '${container.name}')">
                            ⏸ Detener
                        </button>
                    ` : `
                        <button class="btn btn-success btn-sm" onclick="startContainer('${container.id}', '${container.name}')">
                            ▶ Iniciar
                        </button>
                    `}
                    <button class="btn btn-primary btn-sm" onclick="restartContainer('${container.id}', '${container.name}')">
                        ↻ Reiniciar
                    </button>
                    <button class="btn btn-logs btn-sm" onclick="showLogs('${container.id}', '${container.name}')">
                        📋 Logs
                    </button>
                    <button class="btn btn-danger btn-sm" onclick="deleteContainer('${container.id}', '${container.name}')">
                        🗑️ Eliminar
                    </button>
                ` : `
                    <button class="btn btn-logs btn-sm" onclick="showLogs('${container.id}', '${container.name}')">
                        📋 Logs
                    </button>
                    <div class="info-text">ℹ️ Contenedor del sistema (solo informativo)</div>
                `}
            </div>
        </div>
        `;
    }).join('');
    
    containersList.innerHTML = allContainersHTML;
}

// Reiniciar contenedor
async function restartContainer(id, name) {
    if (!confirm(`¿Reiniciar el contenedor "${name}"?`)) {
        return;
    }

    try {
        const response = await fetch(`${API_URL}/containers/${id}/restart`, {
            method: 'POST'
        });

        if (!response.ok) {
            throw new Error('Error al reiniciar el contenedor');
        }

        showNotification(`✓ Contenedor "${name}" reiniciado`, 'success');
        setTimeout(loadContainers, 2000);
    } catch (error) {
        console.error('Error:', error);
        showNotification(`✗ Error: ${error.message}`, 'error');
    }
}

// Iniciar contenedor
async function startContainer(id, name) {
    try {
        const response = await fetch(`${API_URL}/containers/${id}/start`, {
            method: 'POST'
        });

        if (!response.ok) {
            throw new Error('Error al iniciar el contenedor');
        }

        showNotification(`✓ Contenedor "${name}" iniciado`, 'success');
        setTimeout(loadContainers, 2000);
    } catch (error) {
        console.error('Error:', error);
        showNotification(`✗ Error: ${error.message}`, 'error');
    }
}

// Detener contenedor
async function stopContainer(id, name) {
    if (!confirm(`¿Detener el contenedor "${name}"?`)) {
        return;
    }

    try {
        const response = await fetch(`${API_URL}/containers/${id}/stop`, {
            method: 'POST'
        });

        if (!response.ok) {
            throw new Error('Error al detener el contenedor');
        }

        showNotification(`✓ Contenedor "${name}" detenido`, 'success');
        setTimeout(loadContainers, 2000);
    } catch (error) {
        console.error('Error:', error);
        showNotification(`✗ Error: ${error.message}`, 'error');
    }
}

// Eliminar contenedor
async function deleteContainer(id, name) {
    if (!confirm(`⚠️ ¿ELIMINAR PERMANENTEMENTE el contenedor "${name}"?\n\nEsta acción no se puede deshacer.`)) {
        return;
    }

    try {
        const response = await fetch(`${API_URL}/containers/${id}/delete`, {
            method: 'POST'
        });

        if (!response.ok) {
            throw new Error('Error al eliminar el contenedor');
        }

        showNotification(`✓ Contenedor "${name}" eliminado`, 'success');
        setTimeout(loadContainers, 1000);
    } catch (error) {
        console.error('Error:', error);
        showNotification(`✗ Error: ${error.message}`, 'error');
    }
}

// Mostrar logs
async function showLogs(id, name) {
    const modal = document.getElementById('logsModal');
    const logsContent = document.getElementById('logsContent');
    const logsTitle = document.getElementById('logsTitle');

    logsTitle.textContent = `Logs de ${name}`;
    logsContent.textContent = 'Cargando logs...';
    modal.classList.add('active');

    try {
        const response = await fetch(`${API_URL}/containers/${id}/logs`);
        
        if (!response.ok) {
            throw new Error('Error al obtener los logs');
        }

        const logs = await response.text();
        logsContent.textContent = logs || 'No hay logs disponibles';
    } catch (error) {
        console.error('Error:', error);
        logsContent.textContent = `Error al cargar los logs: ${error.message}`;
    }
}

// Cerrar modal
function closeLogsModal() {
    document.getElementById('logsModal').classList.remove('active');
}

// Cerrar modal al hacer click fuera
window.onclick = function(event) {
    const modal = document.getElementById('logsModal');
    if (event.target === modal) {
        closeLogsModal();
    }
    const confirmModal = document.getElementById('confirmModal');
    if (event.target === confirmModal) {
        closeConfirmModal();
    }
}

// Mostrar notificación (simplificada - puedes mejorarla con toast notifications)
function showNotification(message, type) {
    alert(message);
}

// ============ PROCESOS ============

// Cargar procesos
async function loadProcesses() {
    try {
        const response = await fetch(`${API_URL}/processes`);
        
        if (!response.ok) {
            throw new Error(`Error HTTP: ${response.status}`);
        }
        
        const processes = await response.json();
        renderProcesses(processes);
    } catch (error) {
        console.error('Error al cargar procesos:', error);
        const processesList = document.getElementById('processesList');
        processesList.innerHTML = `
            <tr>
                <td colspan="5" class="error-row">Error al cargar procesos: ${error.message}</td>
            </tr>
        `;
    }
}

// Renderizar procesos
function renderProcesses(processes) {
    const processesList = document.getElementById('processesList');
    const processesCount = document.getElementById('processes-count');
    
    if (!processes || processes.length === 0) {
        processesList.innerHTML = `
            <tr>
                <td colspan="5" class="empty-row">No hay procesos para mostrar</td>
            </tr>
        `;
        processesCount.textContent = '0 procesos';
        return;
    }
    
    processesCount.textContent = `${processes.length} procesos`;
    
    processesList.innerHTML = processes.map(proc => `
        <tr>
            <td><span class="pid-badge">${proc.pid}</span></td>
            <td>${proc.user}</td>
            <td><span class="cpu-usage ${proc.cpu > 50 ? 'high' : ''}">${proc.cpu}%</span></td>
            <td><span class="mem-usage ${proc.mem > 50 ? 'high' : ''}">${proc.mem}%</span></td>
            <td class="cmd-cell">${proc.command}</td>
        </tr>
    `).join('');
}

// ============ ACCIONES DEL SISTEMA ============

let currentAction = null;

// Configurar acciones del sistema
function setupSystemActions() {
    const rebootBtn = document.getElementById('rebootBtn');
    const updateBtn = document.getElementById('updateBtn');
    
    if (rebootBtn && !rebootBtn.hasAttribute('data-listener')) {
        rebootBtn.setAttribute('data-listener', 'true');
        rebootBtn.addEventListener('click', () => {
            showConfirmModal(
                'Reiniciar Sistema',
                '¿Estás seguro de que quieres reiniciar la Orange Pi? Todos los servicios se detendrán temporalmente.',
                'reboot'
            );
        });
    }
    
    if (updateBtn && !updateBtn.hasAttribute('data-listener')) {
        updateBtn.setAttribute('data-listener', 'true');
        updateBtn.addEventListener('click', () => {
            showConfirmModal(
                'Actualizar Sistema',
                '¿Deseas actualizar el sistema? Se ejecutará "apt update && apt upgrade -y". Esto puede tardar varios minutos.',
                'update'
            );
        });
    }
}

// Mostrar modal de confirmación
function showConfirmModal(title, message, action) {
    currentAction = action;
    document.getElementById('confirmTitle').textContent = title;
    document.getElementById('confirmMessage').textContent = message;
    document.getElementById('confirmModal').classList.add('active');
    
    const confirmBtn = document.getElementById('confirmActionBtn');
    confirmBtn.onclick = () => executeSystemAction(action);
}

// Cerrar modal de confirmación
function closeConfirmModal() {
    document.getElementById('confirmModal').classList.remove('active');
    currentAction = null;
}

// Ejecutar acción del sistema
async function executeSystemAction(action) {
    closeConfirmModal();
    
    const actionBtn = action === 'reboot' ? document.getElementById('rebootBtn') : document.getElementById('updateBtn');
    const originalHTML = actionBtn.innerHTML;
    
    actionBtn.disabled = true;
    actionBtn.innerHTML = '<span class="spinner-sm"></span> Procesando...';
    
    try {
        const response = await fetch(`${API_URL}/system/${action}`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            throw new Error(`Error HTTP: ${response.status}`);
        }
        
        const result = await response.json();
        
        if (action === 'reboot') {
            showNotification('Sistema reiniciándose... La conexión se perderá momentáneamente.', 'info');
        } else {
            showNotification(result.message || 'Actualización iniciada correctamente', 'success');
        }
    } catch (error) {
        console.error('Error:', error);
        showNotification(`Error al ejecutar la acción: ${error.message}`, 'error');
    } finally {
        actionBtn.disabled = false;
        actionBtn.innerHTML = originalHTML;
    }
}
