package main

import (
	"errors"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
)

type CmdType = byte

const (
	COMMAND_UNKNOWN CmdType = 0x00
	COMMAND_INLINE  CmdType = 0x01
	COMMAND_BULK    CmdType = 0x02
)

const (
	GODIS_IO_BUF     int = 1024 * 16
	GODIS_MAX_BULK   int = 1024 * 4
	GODIS_MAX_INLINE int = 1024 * 4
)

type GodisDB struct {
	data   *Dict
	expire *Dict
}

type GodisServer struct {
	fd      int
	port    int
	db      *GodisDB
	clients map[int]*GodisClient
	aeLoop  *AeLoop
}

type GodisClient struct {
	fd      int
	db      *GodisDB
	args    []*Gobj
	reply   *List
	sentLen int
	// 每次write可能发送不完
	queryBuf []byte
	queryLen int
	cmdTy    CmdType // 命令行类型
	bulkNum  int     // bulk数组长度
	bulkLen  int     // bulk数组每个元素的长度
}

type GodisCommand struct {
	name  string
	proc  CommandProc
	arity int
}

type CommandProc func(c *GodisClient)

// 全局变量
var server GodisServer
var cmdTable []GodisCommand = []GodisCommand{
	//todo{"get"}
}

func getCommand(c *GodisClient) {
	// key := c.args[1]
	// val := findKeyRead(key)
}

func ReadQueryFromClient(loop *AeLoop, fd int, extra interface{}) {
	client := extra.(*GodisClient)
	// queryBuf指向未处理的开头，queryLen为长度
	// 扩容
	if len(client.queryBuf)-client.queryLen < GODIS_MAX_BULK {
		client.queryBuf = append(client.queryBuf, make([]byte, GODIS_MAX_BULK)...)
	}
	n, err := Read(fd, client.queryBuf[client.queryLen:])
	if err != nil {
		log.Printf("client %v read err: %v\n", fd, err)
		freeClient(client)
		return
	}
	client.queryLen += n
	log.Printf("read %v bytes from client: %v\n", n, client.fd)
	log.Printf("ReadQueryFromClient, queryBuf: %v\n", string(client.queryBuf))
	err = ProcessQueryBuf(client)
	if err != nil {
		log.Printf("process query buf err: %v\n", err)
		freeClient(client)
		return
	}
}

func SendReplyToClient(loop *AeLoop, fd int, extra interface{}) {
	client := extra.(*GodisClient)
	log.Printf("SendReplyToClient, reply len:%v\n", client.reply.Length())
	// 元素不会空，就循环
	for client.reply.Length() > 0 {
		rep := client.reply.First()
		buf := []byte(rep.Val.StrVal())
		bufLen := len(buf)
		if client.sentLen < bufLen {
			n, err := Write(fd, buf[client.sentLen:])
			if err != nil {
				log.Printf("send reply err: %v\n", err)
				freeClient(client)
				return
			}
			client.sentLen += n
			log.Printf("send %v bytes to client %v\n", n, client.fd)
			if client.sentLen == bufLen {
				client.reply.DelNode(rep)
				rep.Val.DecrRefCount()
				client.sentLen = 0
			} else {
				// 可写缓冲区满了
				break
			}
		}
	}

	if client.reply.Length() == 0 {
		client.sentLen = 0
		loop.RemoveFileEvent(fd, AE_WRITABLE)
	}
}

func GStrEqual(a, b *Gobj) bool {
	if a.Type_ != GSTR || b.Type_ != GSTR {
		return false
	}
	return a.StrVal() == b.StrVal()
}

func GStrHash(key *Gobj) int64 {
	if key.Type_ != GSTR {
		return 0
	}
	hash := fnv.New64()
	hash.Write([]byte(key.StrVal()))
	return int64(hash.Sum64())
}

func freeClient(client *GodisClient) {
	freeArgs(client)
	delete(server.clients, client.fd)
	server.aeLoop.RemoveFileEvent(client.fd, AE_READABLE)
	server.aeLoop.RemoveFileEvent(client.fd, AE_WRITABLE)
	freeReplyList(client)
	Close(client.fd)
}

func freeArgs(client *GodisClient) {
	for _, v := range client.args {
		v.DecrRefCount()
	}
}

func freeReplyList(client *GodisClient) {
	for client.reply.length != 0 {
		n := client.reply.head
		client.reply.DelNode(n)
		n.Val.DecrRefCount()
	}
}

func ProcessQueryBuf(client *GodisClient) error {
	for client.queryLen > 0 {
		if int(client.cmdTy) == int(COMMAND_UNKNOWN) {
			// RESP Arrays 以*开头，然后跟着数组的长度，元素可以是不同的组合
			// client端向server端发送命令使用的是RESP array
			if client.queryBuf[0] == '*' {
				client.cmdTy = COMMAND_BULK
			} else {
				client.cmdTy = COMMAND_INLINE
			}
		}
		// 将query转为args
		var ok bool
		var err error
		if client.cmdTy == COMMAND_INLINE {
			ok, err = handleInlineBuf(client)
		} else if client.cmdTy == COMMAND_BULK {
			ok, err = handleBulkBuf(client)
		} else {
			return errors.New("unknown Godis Command Type")
		}
		if err != nil {
			return err
		}
		if ok {
			if len(client.args) == 0 {
				// 命令已经处理完了，空命令
				resetClient(client)
			} else {
				// todo ProcessCommand(client)
			}
		} else {
			// 说明当前没有读取完全，等下次再处理
			break
		}
	}
	return nil
}

func handleInlineBuf(client *GodisClient) (bool, error) {
	index, err := client.findLineInQuery()
	if index < 0 {
		return false, err
	}

	subs := strings.Split(string(client.queryBuf[:index]), " ")
	client.queryBuf = client.queryBuf[index+2:]
	client.queryLen -= index + 2
	client.args = make([]*Gobj, len(subs))
	for i, v := range subs {
		// 第一次创建obj
		client.args[i] = CreateObject(GSTR, v)
	}

	return true, nil
}

func (client *GodisClient) findLineInQuery() (int, error) {
	index := strings.Index(string(client.queryBuf[:client.queryLen]), "\r\n")
	if index < 0 && client.queryLen > GODIS_MAX_INLINE {
		return index, errors.New("too big inline cmd")
	}
	return index, nil
}

func handleBulkBuf(client *GodisClient) (bool, error) {
	// 首先处理，数组里多少个数字
	if client.bulkNum == 0 {
		index, err := client.findLineInQuery()
		if index < 0 {
			return false, err
		}

		bnum, err := client.getNumInQuery(1, index)
		if err != nil {
			return false, err
		}
		if bnum == 0 {
			return true, nil
		}
		client.bulkNum = bnum
		client.args = make([]*Gobj, bnum)
	}
	// 每个数组元素的长度
	for client.bulkNum > 0 {
		if client.bulkLen == 0 {
			index, err := client.findLineInQuery()
			if err != nil {
				return false, err
			}

			if client.queryBuf[0] != '$' {
				return false, errors.New("expect $ for bulk length")
			}

			blen, err := client.getNumInQuery(1, index)
			if err != nil || blen == 0 {
				return false, err
			}
			if blen > GODIS_MAX_BULK {
				return false, errors.New("too big bulk")
			}
			client.bulkLen = blen
		}
		// 读取bulk字符串
		if client.queryLen < client.bulkLen+2 {
			return false, nil
		}
		index := client.bulkLen
		if client.queryBuf[index] != '\r' || client.queryBuf[index+1] != '\n' {
			return false, errors.New("expect CRLF for bulk end")
		}
		client.args[len(client.args)-client.bulkNum] = CreateObject(GSTR, string(client.queryBuf[:index]))
		client.queryBuf = client.queryBuf[index+2:]
		client.queryLen -= index + 2
		client.bulkLen = 0
		client.bulkNum--
	}
	return true, nil
}

func (client *GodisClient) getNumInQuery(s, e int) (int, error) {
	num, err := strconv.Atoi(string(client.queryBuf[s:e]))
	client.queryBuf = client.queryBuf[e+2:]
	client.queryLen -= e + 2
	return num, err
}

func resetClient(client *GodisClient) {
	freeArgs(client)
	client.cmdTy = COMMAND_UNKNOWN
	client.bulkLen = 0
	client.bulkNum = 0
}

func ProcessCommand(c *GodisClient) {
	cmdStr := c.args[0].StrVal()
	log.Printf("process command: %v\n", cmdStr)
	if cmdStr == "quit" {
		freeClient(c)
		return
	}
	// todo lookupCommand(cmdStr)
}
