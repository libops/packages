ARG BASE_IMAGE
FROM utility AS installer
FROM ${BASE_IMAGE} AS fixture
RUN apk add --update-cache abuild build-base && \
    mkdir -p /root/.abuild && \
    printf 'PACKAGER="LibOps <install-fixture@keys.libops.invalid>"\n' > /root/.abuild/abuild.conf && \
    abuild-keygen -an && cp /root/.abuild/*.rsa.pub /etc/apk/keys/
COPY --from=installer /out/packages /usr/local/bin/packages
WORKDIR /fixture/main/nginx
COPY ci/install-fixture/APKBUILD ./APKBUILD
RUN abuild -F -r && mkdir /packages && \
    cp /root/packages/main/*/*.apk /packages/ && \
    apk index -o /packages/APKINDEX.tar.gz /packages/*.apk && \
    abuild-sign /packages/APKINDEX.tar.gz && apk verify /packages/APKINDEX.tar.gz && \
    mkdir /packages/keys && cp /etc/apk/keys/*.pub /packages/keys/ && \
    packages write-installers /packages nginx
FROM scratch
COPY --from=fixture /packages /packages
COPY LICENSE NOTICE /packages/
COPY --from=fixture /etc/apk/keys /keys
