FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /cairn-api ./cmd/cairn-api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /cairn-api /cairn-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/cairn-api"]
