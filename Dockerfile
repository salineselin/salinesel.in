# build a container with both gcsfuse and hugo
FROM amd64/debian:stable

# I could have used apt-get to install both gcsfuse and hugo, but I prefer to pull the releases directly from github 
ENV GCSFUSE_VERSION 0.27.0
ENV HUGO_VERSION 0.100.2
RUN apt-get update \
    && apt-get -y install curl fuse \
    && curl -LO https://github.com/GoogleCloudPlatform/gcsfuse/releases/download/v${GCSFUSE_VERSION}/gcsfuse_${GCSFUSE_VERSION}_amd64.deb \
    && curl -LO https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_${HUGO_VERSION}_Linux-64bit.deb \
    && dpkg --install hugo_${HUGO_VERSION}_Linux-64bit.deb \
    && dpkg --install gcsfuse_${GCSFUSE_VERSION}_amd64.deb \
    && rm -Rf gcsfuse_${GCSFUSE_VERSION}_amd64.deb hugo_{HUGO_VERSION}_Linux-64bit.deb

ENTRYPOINT ["gcsfuse", "-o", "allow_other", "--foreground", "--implicit-dirs", "/gcs"]