FROM node:20-alpine AS base
RUN npm install -g pnpm@10.28.0

FROM base AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
ARG NEXT_PUBLIC_MAPTILER_API_KEY
ENV NEXT_PUBLIC_MAPTILER_API_KEY=$NEXT_PUBLIC_MAPTILER_API_KEY
ARG NEXT_PUBLIC_TILES_URL
ENV NEXT_PUBLIC_TILES_URL=$NEXT_PUBLIC_TILES_URL
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ARG SITE_URL
ENV SITE_URL=$SITE_URL
# Destination of the /api/* rewrite (see next.config.ts). Set at build time as
# well as at runtime: Next may resolve rewrites while building the routes
# manifest, so the value has to be correct in the builder too, not just when the
# standalone server starts.
ARG API_INTERNAL_URL
ENV API_INTERNAL_URL=$API_INTERNAL_URL
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN test -n "$SITE_URL"
RUN test -n "$API_INTERNAL_URL"
RUN pnpm build

FROM base AS runner
WORKDIR /app
ENV NODE_ENV=production
ARG SITE_URL
ENV SITE_URL=$SITE_URL
# Also needed at runtime: the standalone server re-reads next.config when it
# boots, and API_INTERNAL_URL keeps the /api/* rewrite pointing at the compose
# service name rather than the published loopback port.
ARG API_INTERNAL_URL
ENV API_INTERNAL_URL=$API_INTERNAL_URL
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["node", "server.js"]