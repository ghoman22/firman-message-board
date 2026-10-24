FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /message-board .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /message-board /message-board
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/message-board"]
