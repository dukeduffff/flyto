# flyto
内网穿透工具

# protobuf生成go 代码
```shell
protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative flyto.proto
```

# 使用教程

## 服务端
启动服务端，并监听7000端口
```shell
flyto -m s -s 7000
```

## 客户端
启动客户端，并连接到服务端7000端口，并将本地服务的6666端口映射到服务端的6666端口
```shell
./flyto -m c -r 192.168.1.88:7000 -l 127.0.0.1:6666:6666
```