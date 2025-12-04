FROM ghcr.io/dtzar/helm-kubectl:3.19.1
SHELL [ "/bin/bash", "-c" ]

RUN addgroup -g 2001 clovers && adduser -D -G clovers -u 2001 clovers


RUN echo "https://mirrors.aliyun.com/alpine/v3.20/main/" >> /etc/apk/repositories \
    && apk update \
    && apk add wget gcc g++ make bind-tools \
    && apk upgrade libexpat \
    && apk add --no-cache ca-certificates curl git jq yq openssl vim bash-completion \
    && apk add --no-cache k9s \
    && apk add --no-cache ttyd 
    
#RUN curl -LsSf https://github.com/helmfile/helmfile/releases/download/v1.2.2/helmfile_1.2.2_darwin_arm64.tar.gz | tar -C /usr/local/bin/ -xzf- helmfile \
#    && chmod 755 /usr/local/bin/helmfile

USER clovers
WORKDIR /home/clovers


COPY ./scripts/.bashrc /home/clovers/
COPY ./scripts/.vimrc /home/clovers/


EXPOSE 7681
CMD [ "ttyd", "-p", "7681", "-i","0.0.0.0" ,"/bin/bash" ]