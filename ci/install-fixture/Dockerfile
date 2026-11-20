ARG BASE_IMAGE
FROM ${BASE_IMAGE} AS fixture
RUN apk add --update-cache abuild build-base && \
    mkdir -p /root/.abuild && \
    printf 'PACKAGER="LibOps <install-fixture@keys.libops.invalid>"\n' > /root/.abuild/abuild.conf && \
    abuild-keygen -an && cp /root/.abuild/*.rsa.pub /etc/apk/keys/
WORKDIR /fixture/main/nginx
COPY ci/install-fixture/APKBUILD ./APKBUILD
RUN abuild -F -r && mkdir /packages && \
    cp /root/packages/main/*/*.apk /packages/ && \
    apk index -o /packages/APKINDEX.tar.gz /packages/*.apk && \
    abuild-sign /packages/APKINDEX.tar.gz && apk verify /packages/APKINDEX.tar.gz
FROM scratch
COPY --from=fixture /packages /packages
COPY --from=fixture /etc/apk/keys /keys
