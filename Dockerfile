ARG VER="0.6.0"
ARG ARCH="amd64"
ARG OS="linux"

ARG BUILDER_IMAGE="golang"
ARG BUILDER_VER="1.26-alpine"

FROM "${BUILDER_IMAGE}:${BUILDER_VER}" AS builder

ARG VER
ARG ARCH
ARG OS

ENV SRC_PATH="/build/hostpath-provisioner"

RUN apk --no-cache add git && \
    mkdir -p "${SRC_PATH}"

ADD . "${SRC_PATH}"

ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS="${OS}"
ENV GOARCH="${ARCH}"
WORKDIR "${SRC_PATH}"
RUN go build -a -ldflags '-extldflags "-static"' -o /hostpath-provisioner

FROM scratch

ARG VER

LABEL ORG="ArkCase LLC" \
      MAINTAINER="Armedia Devops Team <devops@armedia.com>" \
      APP="ArkCase Hostpath Provisioner for Kubernetes" \
      VERSION="${VER}"

COPY --from=builder /hostpath-provisioner /hostpath-provisioner

ENTRYPOINT [ "/hostpath-provisioner" ]
