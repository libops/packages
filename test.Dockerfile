ARG BASE_IMAGE
FROM ${BASE_IMAGE}
ARG PACKAGE
RUN --mount=type=bind,from=packages,source=/packages,target=/packages \
    --mount=type=bind,from=packages,source=/keys,target=/carrier-keys \
    cp /carrier-keys/*.pub /etc/apk/keys/ && \
    apk verify /packages/APKINDEX.tar.gz && \
    case "$PACKAGE" in \
      php*) set -- /packages/${PACKAGE}-[0-9]*.apk /packages/${PACKAGE}-common-*.apk ;; \
      imagemagick) set -- /packages/imagemagick-[0-9]*.apk /packages/imagemagick-libs-*.apk ;; \
      openjdk*|python*) set -- /packages/${PACKAGE}-*.apk ;; \
      *) set -- /packages/${PACKAGE}-[0-9]*.apk ;; \
    esac && \
    apk add --no-cache --repository /packages/APKINDEX.tar.gz "$@" && \
    case "$PACKAGE" in \
      nginx) nginx -v ;; \
      php*) "$PACKAGE" -v ;; \
      openjdk*) java -version && javac -version ;; \
      python*) python3 -c 'import ssl, sqlite3, ctypes, ensurepip' ;; \
      ruby*) ruby -ropenssl -rpsych -e 'puts RUBY_VERSION' && gem --version && bundle --version ;; \
      go*) go version && go env GOROOT ;; \
      imagemagick) magick -version ;; \
      leptonica) apk info -e leptonica ;; \
      *) exit 1 ;; \
    esac && \
    grep -Eq "^${PACKAGE}[^>]*><Q1" /etc/apk/world
