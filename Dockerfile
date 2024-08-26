ARG BUILDER_IMAGE="golang"
ARG BUILDER_VER="1.22-alpine3.19"
ARG ARCH="amd64"
ARG OS="linux"
ARG VER="0.5.0"

FROM "${BUILDER_IMAGE}:${BUILDER_VER}" AS builder

ARG SRCPATH="/build/hostpath-provisioner"

RUN apk --no-cache add git && \
    mkdir -p "${SRCPATH}"

ADD . "${SRCPATH}"

RUN cd "${SRCPATH}" && \
    GO111MODULE=on \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -ldflags '-extldflags "-static"' -o /hostpath-provisioner

FROM scratch

ARG VER

LABEL ORG="ArkCase LLC" \
      MAINTAINER="Armedia Devops Team <devops@armedia.com>" \
      APP="ArkCase Hostpath Provisioner for Kubernetes" \
      VERSION="${VER}"

COPY --from=builder /hostpath-provisioner /hostpath-provisioner

ENTRYPOINT [ "/hostpath-provisioner" ]
