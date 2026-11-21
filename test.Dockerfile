ARG BASE_IMAGE
FROM ${BASE_IMAGE}
ARG PACKAGE
RUN --mount=type=bind,from=packages,source=/packages,target=/packages \
    /packages/"$PACKAGE" && \
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
