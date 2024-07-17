FROM golang:1.21.6-alpine3.19 AS BUILDER

RUN apk update && apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" \
    go build -buildvcs -o /usr/bin/tahawul

FROM alpine:3.19 AS RUNNER

COPY --from=BUILDER /usr/bin/tahawul /usr/bin/tahawul

EXPOSE 8080
ENTRYPOINT [ "tahawul" ]
