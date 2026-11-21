ARG BASE_IMAGE
ARG GO_IMAGE=golang:1.27.4-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280
FROM ${GO_IMAGE} AS utility
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY buildhelpers ./buildhelpers
RUN go mod vendor && \
    CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/packages ./cmd/packages && \
    find vendor -type f \( -iname 'license*' -o -iname 'notice*' -o -iname 'copying*' -o -name COPYRIGHT \) \
      -exec sh -c 'install -Dm644 "$1" "/out/notices/$1"' _ {} \;

FROM ${BASE_IMAGE} AS tools
ARG BASE_IMAGE
ENV PACKAGES_BASE_IMAGE=${BASE_IMAGE}
RUN test -n "$BASE_IMAGE" && apk add --no-cache alpine-sdk sudo git gnupg && \
    adduser -D -G abuild builder && \
    echo 'builder ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/builder
COPY --from=utility /out/packages /usr/local/bin/packages
COPY --from=utility /out/notices /usr/share/licenses/libops-packages/dependencies
COPY --from=utility /usr/local/go/LICENSE /usr/share/licenses/go/LICENSE
COPY LICENSE /usr/share/licenses/libops-packages/LICENSE
USER builder
WORKDIR /home/builder/aports/package
