FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/proxy .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/proxy /usr/local/bin/proxy
ENV PORT=8080
EXPOSE 8080
CMD ["/usr/local/bin/proxy"]
