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
	assert.Nil(t, cfd)
	msg := "helloworld"
	n, err := Write(cfd, []byte(msg))
	assert.Nil(t, err)
	assert.Equal(t, 10, n)
	buf := make([]byte, 10)
	n, err = Read(cfd, buf)
	assert.
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
