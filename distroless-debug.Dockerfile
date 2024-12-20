FROM alpine AS build-env
RUN apk add --update --no-cache mailcap

FROM gcr.io/distroless/static-debian12:debug

LABEL maintainer="The Perses Authors <perses-team@googlegroups.com>"

USER nobody

COPY --chown=nobody:nobody bin/manager                   /bin/manager
COPY --chown=nobody:nobody LICENSE                       /LICENSE
COPY --from=build-env --chown=nobody:nobody              /etc/mime.types /etc/mime.types

EXPOSE     8080
ENTRYPOINT [ "/bin/manager" ]
