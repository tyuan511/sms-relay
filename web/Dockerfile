FROM node:22-alpine AS builder

RUN corepack enable && corepack prepare pnpm@11.5.2 --activate

WORKDIR /app

COPY package.json ./
RUN pnpm install --no-frozen-lockfile

COPY . .
RUN pnpm run build

FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
