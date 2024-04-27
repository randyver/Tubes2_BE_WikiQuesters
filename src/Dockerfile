# Gunakan image Go yang sudah ada
FROM golang:latest

COPY go.mod .
COPY go.sum .
COPY server.go .
COPY . .

RUN go mod download

# Kompilasi server.go dan beri nama outputnya server
RUN go build -o server .

# Expose port 8080 ke luar kontainer
EXPOSE 8080

# Command untuk menjalankan server saat kontainer berjalan
CMD ["./server"]
