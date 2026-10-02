FROM node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS base
RUN npm install -g pnpm@10.28.0

FROM base AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
ARG NEXT_PUBLIC_MAPTILER_API_KEY
ENV NEXT_PUBLIC_MAPTILER_API_KEY=$NEXT_PUBLIC_MAPTILER_API_KEY
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ARG SITE_URL
ENV SITE_URL=$SITE_URL
# Destination of the /api/* rewrite (see next.config.ts). Set at build time as
# well as at runtime: Next may resolve rewrites while building the routes
# manifest, so the value has to be correct in the builder too, not just when the
# standalone server starts. In production compose this is http://server:8081.
ARG API_INTERNAL_URL
ENV API_INTERNAL_URL=$API_INTERNAL_URL
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN test -n "$SITE_URL"
RUN test -n "$API_INTERNAL_URL"
RUN pnpm build

# Production runner: plain Node without pnpm or dev dependencies, running as
# the unprivileged `node` user baked into the base image.
FROM node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS runner
WORKDIR /app
ENV NODE_ENV=production
ARG SITE_URL
ENV SITE_URL=$SITE_URL
# Also needed at runtime: the standalone server re-reads next.config when it
# boots, and API_INTERNAL_URL keeps the /api/* rewrite pointing at the compose
# service name rather than the published loopback port.
ARG API_INTERNAL_URL
ENV API_INTERNAL_URL=$API_INTERNAL_URL
COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static
COPY --from=builder --chown=node:node /app/public ./public
USER node
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:3000/ || exit 1
CMD ["node", "server.js"]
