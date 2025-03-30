FROM golang:1.24.1

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY src/ src/
COPY templates/ templates/
COPY localization/ localization/

ENV VIEWS_DIR=/app/internal/views

RUN go build -o /app/build/main /app/src/main.go

EXPOSE 8080

ENV GIN_MODE=release

CMD ["/app/build/main"]