# Build backend binary file
FROM golang:1.27.3-alpine3.24 AS be-builder
ARG RELEASE_BUILD
ARG BUILD_PIPELINE
ARG BUILD_UNIXTIME
ARG BUILD_DATE
ARG CHECK_3RD_API
ARG SKIP_TESTS
ENV RELEASE_BUILD=$RELEASE_BUILD
ENV BUILD_PIPELINE=$BUILD_PIPELINE
ENV BUILD_UNIXTIME=$BUILD_UNIXTIME
ENV BUILD_DATE=$BUILD_DATE
ENV CHECK_3RD_API=$CHECK_3RD_API
ENV SKIP_TESTS=$SKIP_TESTS
WORKDIR /build
COPY . .
RUN docker/backend-build-pre-setup.sh
RUN apk add git gcc g++ libc-dev
RUN ./build.sh backend

# Build frontend files
FROM --platform=$BUILDPLATFORM node:26.13.0-alpine3.24 AS fe-builder
ARG RELEASE_BUILD
ARG BUILD_PIPELINE
ARG BUILD_UNIXTIME
ARG BUILD_DATE
ARG BUILD_NODE_OPTIONS
ENV RELEASE_BUILD=$RELEASE_BUILD
ENV BUILD_PIPELINE=$BUILD_PIPELINE
ENV BUILD_UNIXTIME=$BUILD_UNIXTIME
ENV BUILD_DATE=$BUILD_DATE
ENV NODE_OPTIONS=$BUILD_NODE_OPTIONS
WORKDIR /build
COPY . .
RUN docker/frontend-build-pre-setup.sh
RUN apk add git
RUN ./build.sh frontend

# Package docker image
FROM alpine:3.24.3
LABEL org.opencontainers.image.title="Folio" org.opencontainers.image.licenses="MIT"
RUN addgroup -S -g 1000 folio && adduser -S -G folio -u 1000 folio
RUN apk --no-cache add tzdata
COPY docker/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
RUN mkdir -p /folio && chown 1000:1000 /folio \
  && mkdir -p /folio/data && chown 1000:1000 /folio/data \
  && mkdir -p /folio/log && chown 1000:1000 /folio/log \
  && mkdir -p /folio/storage && chown 1000:1000 /folio/storage
WORKDIR /folio
COPY --from=be-builder --chown=1000:1000 /build/folio /folio/folio
COPY --from=fe-builder --chown=1000:1000 /build/dist /folio/public
COPY --chown=1000:1000 conf /folio/conf
COPY --chown=1000:1000 templates /folio/templates
COPY --chown=1000:1000 LICENSE /folio/LICENSE
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
