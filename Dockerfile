FROM alpine AS build-env
RUN apk add --update --no-cache mailcap

FROM gcr.io/distroless/static-debian12
ARG TARGETPLATFORM
LABEL maintainer="The Perses Authors <perses-team@googlegroups.com>"

USER 65532:65532

COPY --chown=65532:65532                               ${TARGETPLATFORM}/bin/manager    /bin/manager
COPY --chown=65532:65532                               LICENSE                          /LICENSE
COPY --from=build-env --chown=65532:65532              /etc/mime.types                  /etc/mime.types

EXPOSE     8080
ENTRYPOINT [ "/bin/manager" ]
