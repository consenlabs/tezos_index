FROM registry.cn-hongkong.aliyuncs.com/imtoken/token-lua:master AS openresty-env

RUN mkdir /app

COPY ./* /app/

ENTRYPOINT [ "/app/tezos_index" ]