# Usar una imagen base de Golang
FROM golang:1.20

# Establecer el directorio de trabajo
WORKDIR /app

# Copiar los archivos Go
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Construir el binario
RUN go build -o server .

# Exponer el puerto
EXPOSE 3000

# Ejecutar el servidor
CMD ["./server"]
