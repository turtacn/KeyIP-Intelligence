# KeyIP Intelligence - WebUI Quickstart Guide

This guide describes how to build, deploy, and publish the frontend application using an all-in-one approach with Docker and Nginx.

## Prerequisites

- Docker Desktop or Docker Engine installed
- Node.js 20+ (for local development)

## 1. Local Development

To run the frontend locally with mock data:

```bash
cd web
npm install
npm run dev
```

Visit `http://localhost:5173`.

## 2. All-in-One Container Build & Deploy

We use a multi-stage Docker build to compile the React application and serve it via Nginx in a single container image.

### Dockerfile

Ensure you have the `Dockerfile` in the `web` directory (see below).

### Scenario A: Demo Mode (Mock Data / Simulation)

This builds a container that runs **independently of any backend**. It uses the internal Mock Service Worker (MSW) to simulate API responses with realistic test data. This is perfect for demos or testing when the backend is not ready.

**Build:**
```bash
# VITE_API_MODE defaults to 'mock' in the Dockerfile
docker build -t keyip-webui:demo ./web
```

**Run:**
```bash
docker run -d -p 8080:80 --name keyip-demo keyip-webui:demo
```

Access the application at `http://localhost:8080`. You will see full data populated from the mock layer.

### Scenario B: Production Mode (Real Backend)

This builds a container configured to talk to a real backend API.

**Build:**
```bash
# Pass 'real' mode and the API URL
docker build \
  --build-arg VITE_API_MODE=real \
  --build-arg VITE_API_BASE_URL=https://api.keyip.example.com/v1 \
  -t keyip-webui:prod ./web
```

**Run:**
```bash
docker run -d -p 80:80 --name keyip-prod keyip-webui:prod
```

## 3. Modifying Test Data (Simulation)

To customize the simulation data without changing code structure:
1. Edit the JSON files in `web/src/mocks/data/` (e.g., `patents.json`, `alerts.json`).
2. Rebuild the Demo Mode image.

## 4. Publishing to Registry

To publish to a container registry (e.g., Docker Hub, AWS ECR):

```bash
# Tag the image
docker tag keyip-webui:demo myregistry.azurecr.io/keyip-webui:v1.0.0

# Push
docker push myregistry.azurecr.io/keyip-webui:v1.0.0
```

## Appendix: Dockerfile Reference

`web/Dockerfile`:

```dockerfile
# Stage 1: Build
FROM node:20-alpine as builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_API_MODE=mock
ARG VITE_API_BASE_URL
ENV VITE_API_MODE=$VITE_API_MODE
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
RUN npm run build

# Stage 2: Serve
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

## Appendix: Nginx Config (nginx.conf)

`web/nginx.conf`:

```nginx
server {
    listen 80;
    server_name localhost;

    location / {
        root /usr/share/nginx/html;
        index index.html index.htm;
        try_files $uri $uri/ /index.html;
    }
}
```
