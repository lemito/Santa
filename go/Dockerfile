FROM golang:1.23-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends cmake g++ make && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY CMakeLists.txt ./CMakeLists.txt
COPY core ./core
COPY third_party/yasl ./third_party/yasl
RUN cmake -S . -B /src/build -DCMAKE_BUILD_TYPE=Release && cmake --build /src/build --target santa_native
COPY go ./go
WORKDIR /src/go
RUN CGO_ENABLED=1 go build -o /out/santa-go ./cmd/santa-go
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends libstdc++6 ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/santa-go /usr/local/bin/santa-go
COPY --from=build /src/build/libsanta_native.so /usr/local/lib/libsanta_native.so
COPY go/templates /app/templates
COPY go/static /app/static
COPY go/migrations /app/migrations
ENV LD_LIBRARY_PATH=/usr/local/lib
WORKDIR /app
EXPOSE 5080
ENTRYPOINT ["/usr/local/bin/santa-go"]
