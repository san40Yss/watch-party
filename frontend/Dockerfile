# Build the Svelte app, then bake the static output into an nginx image.
# nginx.conf and the media/processed dirs come in via volumes (see compose).
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
