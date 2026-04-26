FROM public.ecr.aws/docker/library/nginx:alpine

# Copiar archivos estáticos
COPY index.html /usr/share/nginx/html/
COPY styles.css /usr/share/nginx/html/
COPY app.js /usr/share/nginx/html/

# Copiar configuración personalizada de nginx (opcional)
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
