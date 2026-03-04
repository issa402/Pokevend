# ============================================================
# PokémonTool — React Frontend Dockerfile
# Builds the static bundle then serves via nginx
# ============================================================

FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:alpine AS runner
# Remove default nginx page
RUN rm -rf /usr/share/nginx/html/*
COPY --from=builder /app/dist /usr/share/nginx/html
# nginx.conf to handle React Router (client-side routing)
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
