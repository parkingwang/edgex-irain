FROM scratch

# 执行文件名称，须与 name.txt 中一致
COPY endpoint-irain /bin/

ONBUILD COPY application.toml /etc/edgex/

CMD ["endpoint-irain"]