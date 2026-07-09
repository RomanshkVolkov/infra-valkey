# ---- build stage ----------------------------------------------------
FROM golang:1.23-alpine AS build
WORKDIR /src

# Cache de dependencias: copia primero los módulos.
COPY go.mod go.sum* ./
# Si go.sum no existe aún, genera el grafo de módulos.
RUN go mod download || true

COPY . .
# Binario estático (CGO off) para correr en distroless.
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=mod \
    go build -ldflags="-s -w" -o /out/app-ejemplo .

# ---- runtime stage --------------------------------------------------
# distroless static: sin shell, sin package manager => superficie mínima.
# El usuario 'nonroot' es UID 65532 (coincide con el securityContext del Deployment).
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/app-ejemplo /app/app-ejemplo
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/app-ejemplo"]
