# This dockerfile should be built with `make docker-build` from the stash root.

# Build Frontend
FROM node:24-alpine AS frontend
RUN apk add --no-cache make git
## cache node_modules separately
COPY ./ui/v2.5/package.json ./ui/v2.5/pnpm-lock.yaml /stash/ui/v2.5/
WORKDIR /stash
COPY Makefile /stash/
COPY ./graphql /stash/graphql/
COPY ./ui /stash/ui/
# pnpm install with npm
RUN npm install -g pnpm@10.33.0
RUN make pre-ui
RUN make generate-ui
ARG GITHASH
ARG STASH_VERSION
RUN BUILD_DATE=$(date +"%Y-%m-%d %H:%M:%S") make ui-only

# Build Backend
FROM golang:1.25.9-alpine AS backend
RUN apk add --no-cache make alpine-sdk sqlite-dev
WORKDIR /stash
COPY ./go* ./*.go Makefile gqlgen.yml .gqlgenc.yml /stash/
# sqlite-vec-go-bindings v0.1.6 vendors an older sqlite-vec C snapshot whose
# redundant BSD u_int*_t typedefs fail on musl/Alpine. Apply the upstream
# sqlite-vec musl fix (PR #199) to the module cache before CGO compiles it.
RUN go mod download github.com/asg017/sqlite-vec-go-bindings@v0.1.6 \
    && SQLITE_VEC_C="$(go env GOMODCACHE)/github.com/asg017/sqlite-vec-go-bindings@v0.1.6/cgo/sqlite-vec.c" \
    && chmod u+w "$SQLITE_VEC_C" \
    && awk '\
        /^typedef u_int8_t uint8_t;$/ { print "#ifndef UINT8_MAX" } \
        { print } \
        /^typedef u_int64_t uint64_t;$/ { print "#endif" } \
    ' "$SQLITE_VEC_C" > "$SQLITE_VEC_C.patched" \
    && mv "$SQLITE_VEC_C.patched" "$SQLITE_VEC_C"
COPY ./graphql /stash/graphql/
COPY ./scripts /stash/scripts/
COPY ./pkg /stash/pkg/
COPY ./cmd /stash/cmd/
COPY ./internal /stash/internal/
# needed for generate-login-locale
COPY ./ui /stash/ui/
RUN make generate-backend generate-login-locale
COPY --from=frontend /stash /stash/
ARG GITHASH
ARG STASH_VERSION
RUN make flags-release flags-pie stash

# Final Runnable Image
FROM alpine:latest

RUN apk add --no-cache \
    ca-certificates \
    vips-tools \
    ffmpeg \
    libjxl-tools \
    python3 \
    py3-pip \
    py3-numpy \
    py3-pillow \
    py3-onnxruntime \
    && python3 -m pip install \
        --break-system-packages \
        --no-cache-dir \
        yt-dlp

COPY --from=backend /stash/stash /usr/bin/
COPY --from=backend /stash/scripts/visual_embedding_worker.py /usr/lib/stash/visual_embedding_worker.py
COPY --from=backend /stash/scripts/camie_tagger_worker.py /usr/lib/stash/camie_tagger_worker.py

ENV STASH_CONFIG_FILE=/root/.stash/config.yml
ENV STASH_VISUAL_EMBEDDING_WORKER=/usr/lib/stash/visual_embedding_worker.py
ENV STASH_CAMIE_TAGGER_WORKER=/usr/lib/stash/camie_tagger_worker.py

EXPOSE 9999
ENTRYPOINT ["stash"]
