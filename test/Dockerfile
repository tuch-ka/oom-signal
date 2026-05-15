FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY src/ ./src/
RUN go build -o /oom-signal ./src/cmd

FROM python:3.12-slim

RUN pip install --no-cache-dir psutil

COPY --from=build /oom-signal /usr/local/bin/oom-signal
COPY test/eat_memory.py /opt/oom-signal/eat_memory.py

ENTRYPOINT ["python", "/opt/oom-signal/eat_memory.py"]
CMD ["threads"]
