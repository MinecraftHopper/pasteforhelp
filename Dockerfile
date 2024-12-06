###
# Builder to compile our golang code
###
FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

RUN apk add clang lld
COPY --from=xx / /

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

ARG TARGETPLATFORM

RUN xx-apk add musl-dev gcc
RUN xx-go build -o pasteforhelp -buildvcs=false -v github.com/minecrafthopper/pasteforhelp
RUN xx-verify /build/pasteforhelp

###
# Now generate our smaller image
###
FROM alpine

EXPOSE 8080
ENV STORAGE_DIR="/data-dir"
RUN mkdir ${STORAGE_DIR}

COPY --from=builder /build/pasteforhelp /go/bin/pasteforhelp

ENTRYPOINT ["/go/bin/pasteforhelp"]