package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAe(t *testing.T) {
	loop, err := AeLoopCreate()
	assert.Nil(t, err)
	sfd, err := TcpServer(6666)
	loop.AddFileEvent(sfd, AE_READABLE, AcceptProc, nil)
	go loop.AeMain()
	// 初始化client & 测试文件事件
	host := [4]byte{0, 0, 0, 0}
	cfd, err := Connect(host, 6666)
	assert.Nil(t, err)
	msg := "helloworld"
	n, err := Write(cfd, []byte(msg))
	assert.Nil(t, err)
	assert.Equal(t, 10, n)
	buf := make([]byte, 10)
	n, err = Read(cfd, buf)
	assert.Nil(t, err)
	assert.Equal(t, 10, n)
	// 测试时间事件
	loop.AddTimeEvent(AE_ONCE, 10, OnceProc, t)
	end := make(chan struct{}, 2)
	loop.AddTimeEvent(AE_NORMAL, 10, NormalProc, end)
	<-end
	<-end
	loop.stop = true
}

func AcceptProc(loop *AeLoop, fd int, extra interface{}) {
	cfd, err := Accept(fd)
	if err != nil {
		fmt.Printf("accept err: %v\n", err)
		return
	}
	loop.AddFileEvent(cfd, AE_READABLE, ReadFunc, nil)
}

func ReadFunc(loop *AeLoop, fd int, extra interface{}) {
	buf := make([]byte, 10)
	n, err := Read(fd, buf)
	if err != nil {
		fmt.Printf("read err: %v\n", err)
		return
	}
	fmt.Printf("ae read %v bytes\n", n)
	loop.AddFileEvent(fd, AE_WRITABLE, WriteProc, buf)
}

func WriteProc(loop *AeLoop, fd int, extra interface{}) {
	buf := extra.([]byte)
	n, err := Write(fd, buf)
	if err != nil {
		fmt.Printf("write err: %v\n", err)
		return
	}
	fmt.Printf("ae write %v bytes\n", n)
	loop.RemoveFileEvent(fd, AE_WRITABLE)
}

func OnceProc(loop *AeLoop, id int, extra interface{}) {
	t := extra.(*testing.T)
	assert.Equal(t, 1, id)
	fmt.Printf("time event %v done\n", id)
}

func NormalProc(loop *AeLoop, id int, extra interface{}) {
	end := extra.(chan struct{})
	fmt.Printf("time event %v done\n", id)
	end <- struct{}{}
}
