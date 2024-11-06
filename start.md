### image-transfer

#### 1.运行image-transfer
``` 
go mod tidy
go run cmd/image-transfer/main.go
```

#### 2. UI 界面
``` 
  默认账号: admin
  默认密码: RKO6G6VBH0R5
  http://localhost:8080

```
#### 3.请求接口
```
curl -X POST http://localhost:8080/image-transfer \
-H "Content-Type: application/json" \
-d'{
  "source": {
    "registry.cn-hangzhou.aliyuncs.com": {
        "username": "username",
        "password": "password"
    }

  },
  "target": {
    "swr.cn-east-3.myhuaweicloud.com": {
      "username": "username",
      "password": "password"
    }
  },
  "images":
    {
      "registry.cn-hangzhou.aliyuncs.com/devops/ssh-slave": "swr.cn-east-3.myhuaweicloud.com/devops/ssh-slave",
      "registry.cn-hangzhou.aliyuncs.com/devops/dotnetcore": "swr.cn-east-3.myhuaweicloud.com/devops/dotnetcore"
    }
}'

```

```
返回： message":"Image transfer executed successfully" 成功
```

#### 4. 打包
```
 cd  cmd/image-transfer && CGO_ENABLED=0 GOOS=windows  GOARCH=amd64 go build -o image-transfer.exe  main.go 
 cd cmd/image-transfer && CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -o image-transfer   main.go 
```
