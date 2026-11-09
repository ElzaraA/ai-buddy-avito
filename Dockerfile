FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN go build -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/server /app/server
COPY data /app/data
EXPOSE 8080
CMD ["/app/server"]
