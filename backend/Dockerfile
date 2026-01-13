FROM golang:1.22-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

FROM alpine:3.20

RUN adduser -D -g '' appuser
USER appuser

COPY --from=build /bin/server /bin/server

EXPOSE 8080

ENTRYPOINT ["/bin/server"]
