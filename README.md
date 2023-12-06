# go-redis  
redis整体架构：单线程  
redis架构整体比数据结构细节重要  
书：redis设计与实现  
代码：https://github.com/archeryue/godis  
Redis核心流程  
1.启动流程 main函数做了什么事  
2.请求处理流程(ae库)  
```C++
main() {
  //初始化，核心概念
  //主循环，核心流程；通过ae库来完成主循环
}
```
Redis核心数据结构  
Dict  
结构  
expire  
渐进rehash  
RedisObject  
机制  
set生命周期  
get生命周期  
渐进Rehash，将单线程导致的重操作转化为轻操作  

```Go
//代码结构
ae.go//Archer-Event,epoll相关代码  
conf.go//配置相关文件  
dict.go  
list.go  
zset.go//实现了五个数据结构，string,list,set,dict,zset  
obj.go//redis object相关实现  
net.go//网路库；不使用go协程相关的方法，用大量的系统调用
```