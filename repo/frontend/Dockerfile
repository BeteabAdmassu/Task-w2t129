FROM node:18-alpine AS builder

WORKDIR /build

COPY package.json ./
RUN npm install --legacy-peer-deps --omit=optional && \
    ROLLUP_VER=$(node -e "console.log(require('./node_modules/rollup/package.json').version)") && \
    npm install --no-save "@rollup/rollup-linux-x64-musl@${ROLLUP_VER}"

COPY . .
RUN npm run build

FROM nginx:alpine

COPY --from=builder /build/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 3000

HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
