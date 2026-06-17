FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./
COPY .npmrc ./
RUN npm install --no-audit --no-fund

COPY . .
RUN npm run build:prod

FROM nginx:alpine

COPY --from=builder /app/dist/admin-frontend/browser /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
