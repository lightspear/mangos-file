## 介绍

mangos-file是一款基于mangos-go消息中间件纯go语言研发的专门用于文件传输同步工具
​
[lightspear/mangos-file(gitee)](https://gitee.com/lightspear/mangos-file)
[lightspear/mangos-file(github)](https://github.com/lightspear/mangos-file)

​
## 特点

1. 文件无任何依赖可以编译成linux，window，arm平台都能使用
2. 单文件根据执行参数可以既可以当服务端用也可以当客户端用
3. 支持上传模式和下载模式
4. 支持分时段限速下载
5. 支持服务端多结点资源
6. [下载模式]利用boltdb记录可下载数
7. [下载模式]支持配置优先下载规则如：【PriorRule="<dirs>/%Y%m%d/"】
8. [上传模式]支持配置优先下载规则如：【 PriorRule="<dirs>/%Y%m%d/"】来保证优先上传本天
9. 支持设定文件分片大小
10. 命令行窗口带进度条显示
11. 基于websocket协议非常容易用nginx反向代理容易实现7层负载均衡
12. **支持低速网络，丢包网络下断点续传，来网自动续传，可配置分片slice大小**

## 服务端配置

```bash
#参数执行如下
./mangos-filetransfer.exe -role=server -c server.toml
```

```toml
[App]
Title=""
Port=5000


[ResNodes]

[ResNodes.node1]
RootDir="D:/AllData"
UserName="admin"
UserPwd="123456"


[ResNodes.node2]
RootDir="D:/AllData"
UserName="admin"
UserPwd="123456"

```

## 客户端配置(上传模式)

```bash
#参数执行如下
./mangos-filetransfer.exe -role=client -mode=upload -c client.toml
```

```toml
Title=""
GrpcLiveLogAddr = ""
#远程资源地址
WSServerUrl="ws://127.0.0.1:5000"
#资源节点
ResNode="node1"
#远程目录
RemoteDir="/abc/UploadData"
#本地目录
LocalDir="../测试/store"
#密钥
UserPwd="123456"
#单次下载文件片断大小(单位KB)
SliceSize=128
# #循环执行间隔(单位s)(默认不循环ExecInterval=0)
# ExecInterval=1



#上传优先规则,如果一旦配置则优先上传本天此目录
PriorRule="<dirs>/%Y-%m-%d/"


#规则是依次从上到下匹配，没有匹配则不限速
[[LimitRate]]
#限速时段(如果当前时间不在任何一个时段,那么直接不限速)
TimeRange="07:00~20:00"
#限速大小(单位KB)
Rate=128

```


## 客户端配置(下载模式)

```bash
#参数执行如下
./mangos-filetransfer.exe -role=client -mode=download -c client.toml
```

```toml
Title=""
GrpcLiveLogAddr = ""
#远程资源地址
WSServerUrl="ws://127.0.0.1:5000"
#资源节点
ResNode="node1"
#远程目录
RemoteDir="/abc/UploadData"
#本地目录
LocalDir="../测试/store"
#密钥
UserPwd="123456"
#单次下载文件片断大小(单位KB)
SliceSize=128
# #循环执行间隔(单位s)(默认不循环ExecInterval=0)
# ExecInterval=1





#下载优先规则,如果不配置优先级规则那就是按修改时间优先级
PriorRule="<dirs>/%Y-%m-%d/"
# #起始日期
StartDay="2020-04-01"
# #结束日期
EndDay="2020-12-01"
# 下载记录db
RecordDb="../测试/db/count.db"
#boltdb监控地址可不填
#DbViewHttpPort=9000
# #完成日志文件路径
Completelogfile="../测试/complete/%Y-%m-%d.txt"


#规则是依次从上到下匹配，没有匹配则不限速
[[LimitRate]]
#限速时段(如果当前时间不在任何一个时段,那么直接不限速)
TimeRange="07:00~20:00"
#限速大小(单位KB)
Rate=128



```


## 版本纪要

### 0.0.6版本
修复了DeleteFile书写的位置不好,导致必须配置服务端DeleteFile

### 0.0.8版本
修复了上传模式下低网络的断点续传功能
