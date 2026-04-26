# 🐳 Docker Manager

Aplicación web ligera para gestionar contenedores Docker en tu Orange Pi. Diseñada para ser responsive y consumir mínimos recursos.

## 🎯 Características

- ✅ Ver estado de todos los contenedores Docker
- ▶️ Iniciar/Detener/Reiniciar contenedores
- 📋 Ver logs de contenedores (últimas 100 líneas)
- 📱 Diseño responsive optimizado para móvil
- ⚡ Muy ligero: Backend en Go + Frontend estático
- 🔄 Auto-actualización cada 10 segundos

## 🏗️ Arquitectura

- **Backend**: Go + API REST (se comunica con Docker Engine)
- **Frontend**: HTML/CSS/JavaScript vanilla (sin frameworks)
- **Deployment**: Docker Compose con imágenes Alpine

## 📋 Requisitos

- Docker y Docker Compose instalados en tu Orange Pi
- Mínimo 512MB RAM disponible
- Acceso a `/var/run/docker.sock`

## 🚀 Instalación

### 1. Clonar o copiar el proyecto a tu Orange Pi

```bash
git clone <tu-repo> docker-manager
cd docker-manager
```

### 2. Construir y levantar los contenedores

```bash
docker-compose up -d --build
```

Esto construirá ambas imágenes (backend y frontend) y levantará los contenedores.

### 3. Acceder a la aplicación

Abre tu navegador (desde móvil o PC) y accede a:

```
http://<IP-de-tu-OrangePi>
```

Por ejemplo: `http://192.168.1.100`

## 🔧 Configuración

### Cambiar el puerto del frontend

Edita `docker-compose.yml` y cambia el puerto 80:

```yaml
frontend:
  ports:
    - "3000:80"  # Ahora accedes por el puerto 3000
```

### Cambiar el puerto del backend

Edita `docker-compose.yml`:

```yaml
backend:
  ports:
    - "9090:8080"
  environment:
    - PORT=8080
```

Y actualiza `frontend/app.js` línea 2:

```javascript
const API_URL = `http://${window.location.hostname}:9090/api`;
```

## 📱 Uso desde móvil

1. Asegúrate de estar en la misma red Wi-Fi que tu Orange Pi
2. Accede desde el navegador del móvil: `http://IP-Orange-Pi`
3. Guarda la página en tu pantalla de inicio para acceso rápido

## 🛠️ Comandos útiles

### Ver logs en tiempo real

```bash
# Logs del backend
docker logs -f docker-manager-backend

# Logs del frontend
docker logs -f docker-manager-frontend
```

### Reiniciar la aplicación

```bash
docker-compose restart
```

### Detener la aplicación

```bash
docker-compose down
```

### Actualizar después de cambios

```bash
docker-compose down
docker-compose up -d --build
```

### Ver recursos consumidos

```bash
docker stats docker-manager-backend docker-manager-frontend
```

## 🔒 Seguridad

⚠️ **Importante**: Esta aplicación está diseñada para uso en red local. No expongas directamente a Internet sin:

1. Agregar autenticación (usuario/contraseña)
2. Usar HTTPS/TLS
3. Configurar un firewall

### Agregar autenticación básica (opcional)

Puedes usar autenticación básica de Nginx. Edita `frontend/nginx.conf`:

```nginx
server {
    listen 80;
    
    auth_basic "Docker Manager";
    auth_basic_user_file /etc/nginx/.htpasswd;
    
    # ... resto de configuración
}
```

Y genera el archivo de contraseñas:

```bash
htpasswd -c .htpasswd admin
```

## 🐛 Solución de problemas

### El backend no puede conectar con Docker

Verifica que el socket de Docker esté montado correctamente:

```bash
docker inspect docker-manager-backend | grep docker.sock
```

### El frontend no puede conectar con el backend

1. Verifica que ambos contenedores estén en la misma red
2. Revisa la URL de la API en `frontend/app.js`
3. Comprueba que el puerto 8080 esté abierto

### Contenedores que no aparecen

El backend lista TODOS los contenedores (incluyendo detenidos). Si no aparecen:

```bash
docker ps -a  # Verifica que haya contenedores
docker logs docker-manager-backend  # Revisa logs del backend
```

## 📊 Rendimiento

En una Orange Pi Zero con 512MB RAM:

- Backend: ~15-20 MB RAM
- Frontend (Nginx): ~5-10 MB RAM
- CPU: < 1% en idle
- Tiempo de respuesta: < 100ms

## 🔄 Actualización automática

La interfaz se actualiza automáticamente cada 10 segundos. Puedes cambiar este intervalo en `frontend/app.js` línea 15:

```javascript
setInterval(loadContainers, 30000); // 30 segundos
```

## 🤝 Contribuciones

¿Mejoras? ¡PRs bienvenidos!

## 📝 Licencia

MIT License - Usa y modifica libremente

---

**Hecho con ❤️ para Orange Pi y dispositivos con recursos limitados**
