FROM golang:1.23-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends cmake g++ make pkg-config && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY CMakeLists.txt ./CMakeLists.txt
COPY cmake ./cmake
COPY core ./core
COPY third_party/yasl ./third_party/yasl
RUN cmake -S . -B /src/build -DCMAKE_BUILD_TYPE=Release && cmake --build /src/build --target santa_native
RUN cmake --install /src/build --prefix /usr/local
COPY go ./go
WORKDIR /src/go
RUN PKG_CONFIG_PATH=/usr/local/lib/pkgconfig CGO_ENABLED=1 go build -o /out/santa-go ./cmd/santa-go
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends libstdc++6 ca-certificates tzdata wget && rm -rf /var/lib/apt/lists/* \
    && groupadd --system santa \
    && useradd --system --gid santa --home-dir /app --no-create-home santa
COPY --from=build /out/santa-go /usr/local/bin/santa-go
COPY --from=build /src/build/libsanta_native.so /usr/local/lib/libsanta_native.so
COPY go/templates /app/templates
COPY go/static /app/static
COPY go/migrations /app/migrations
ENV LD_LIBRARY_PATH=/usr/local/lib
WORKDIR /app
USER santa
EXPOSE 5080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD wget -q -O - http://127.0.0.1:5080/health/ready || exit 1
ENTRYPOINT ["/usr/local/bin/santa-go"]
