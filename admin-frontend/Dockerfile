FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com \
    && npm install --no-audit --no-fund

COPY . .
RUN npm run build:prod

FROM nginx:alpine

COPY deployments/nginx/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /app/dist/admin-frontend /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
