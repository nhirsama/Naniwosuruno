# 你现在在干什么？何をしているの。
为了让别人实时可以看到你在做什么，所以编写了这个项目。通过访问 web 页面可以查看你当前运行的窗口名称。

## 编译
因为 `Naniwosuruno` 中第一个字母和最后一个字母之间有10个字母，所以本项目简称为 `n10o`。  
在项目根目录中运行
```bash
go build -o n10o ./main.go
chmod u+x n10o
./n10o
```
将看到程序输出了一段帮助文档，根据需求选择对应的选项即可。

第一次选择任意启动选项将会初始化 token。若客户端和服务端在不同环境运行，则需要修改任意一端中的`./data/config.json`使`token`字段保持相同，并在客户端的配置文件中将 `BaseUrl` 字段填写为服务器域名，如 `http://localhost:9975`。 

### 客户端 
#### Linux 系统 KDE 桌面
目前只适配了 Linux 系统的 KDE 桌面，理论上兼容 x11 和 wayland。  
首先需要编译项目所依赖的 [./client/LinuxKDE/kdotool](https://github.com/jinliu/kdotool/tree/1ad61acb56c0707df53a4d9ce10153a87f08523b)
项目，将其编译生成的可执行文件放置于根目录，并命名为 `kdotool`。  
在项目根目录中运行
```bash
go build -o n10oClient ./client/LinuxKDE/main.go
chmod u+x n10oClient
./n10on10oClient
```
### web
运行服务端之后将会在 [http://localhost:9975](http://localhost:9975)启动服务。