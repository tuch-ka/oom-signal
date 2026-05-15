FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY src/ ./src/
RUN go build -o /oom-signal ./src/cmd

FROM scratch
COPY --from=build /oom-signal /oom-signal
ENTRYPOINT ["/oom-signal"]
