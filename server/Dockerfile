# ============================================================
# PokémonTool — Node.js API Gateway Dockerfile
# Multi-stage build: install deps in stage 1, run in stage 2
# ============================================================

FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --only=production

FROM node:20-alpine AS runner
WORKDIR /app
# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=deps /app/node_modules ./node_modules
COPY . .
USER appuser
EXPOSE 3001
CMD ["node", "index.js"]
