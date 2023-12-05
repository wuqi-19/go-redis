package main

import (
	"log"
	"time"

	"golang.org/x/sys/unix"
)

type FeType int

const (
	AE_READABLE FeType = 1
	AE_WRITABLE FeType = 2
)

type TeType int

const (
	AE_NORMAL TeType = 1
	AE_ONCE   TeType = 2
)

type FileProc func(loop *AeLoop, fd int, extra interface{})
type TimeProc func(loop *AeLoop, id int, extra interface{})

// 文件事件
type AeFileEvent struct {
	fd    int
	mask  FeType   // 文件事件类型
	proc  FileProc // 处理函数
	extra interface{}
}

// 时间事件
type AeTimeEvent struct {
	id       int
	mask     TeType // 时间时间类型
	when     int64  // ms
	interval int64  // ms
	proc     TimeProc
	extra    interface{}
	next     *AeTimeEvent
}

type AeLoop struct {
	// 没有使用Dict结构（主要是有rehash过程），是因为量不会特别大，不会rehash
	FileEvents      map[int]*AeFileEvent // 因为可能有很多个文件事件，所以使用map降低查找复杂度
	TimeEvents      *AeTimeEvent
	fileEventFd     int
	timeEventNextId int
	stop            bool
}

var fe2ep [3]uint32 = [3]uint32{0, unix.EPOLLIN, unix.POLLOUT}

func (loop *AeLoop) AddFileEvent(fd int, mask FeType, proc FileProc, extra interface{}) {
	// 需要添加到epoll和ae里
	ev := loop.getEpollMask(fd)
	if ev&fe2ep[mask] != 0 {
		// 文件事件已经存在
		return
	}

	// op表示操作类型
	op := unix.EPOLL_CTL_ADD
	if ev != 0 {
		op = unix.EPOLL_CTL_MOD
	}
	ev |= fe2ep[mask]
	// 用于向epoll实例中添加、修改或删除文件描述符（fd）关联的事件
	err := unix.EpollCtl(loop.fileEventFd, op, fd, &unix.EpollEvent{Fd: int32(fd), Events: ev})
	if err != nil {
		log.Printf("epoll ctr err: %v\n", err)
		return
	}

	// aeloop中添加文件事件
	fe := AeFileEvent{
		fd:    fd,
		mask:  mask,
		proc:  proc,
		extra: extra,
	}
	loop.FileEvents[getFeKey(fd, mask)] = &fe
	log.Printf("ae add file event fd: %v, mask: %v\n", fd, mask)
}

func (loop *AeLoop) RemoveFileEvent(fd int, mask FeType) {
	// op表示操作类型
	op := unix.EPOLL_CTL_DEL
	ev := loop.getEpollMask(fd)
	// ^表示按位取反，看看取反之后还剩不剩事件
	ev &= ^fe2ep[mask]
	if ev != 0 {
		op = unix.EPOLL_CTL_MOD
	}
	err := unix.EpollCtl(loop.fileEventFd, op, fd, &unix.EpollEvent{Fd: int32(fd), Events: ev})
	if err != nil {
		log.Printf("epoll del err: %v\n", err)
		return
	}

	// aeloop中删除文件事件
	loop.FileEvents[getFeKey(fd, mask)] = nil
	log.Printf("ae remove file event fd: %v, mask: %v\n", fd, mask)
}

func (loop *AeLoop) getEpollMask(fd int) uint32 {
	var ev uint32
	if loop.FileEvents[getFeKey(fd, AE_READABLE)] != nil {
		// 设置为unix.EPOLLIN
		ev |= fe2ep[AE_READABLE]
	}
	if loop.FileEvents[getFeKey(fd, AE_WRITABLE)] != nil {
		// 设置为unix.POLLOUT
		ev |= fe2ep[AE_WRITABLE]
	}
	return ev
}

func getFeKey(fd int, mask FeType) int {
	if mask == AE_READABLE {
		return fd
	} else {
		return fd * -1
	}
}

func (loop *AeLoop) AddTimeEvent(mask TeType, interval int64, proc TimeProc, extra interface{}) int {
	id := loop.timeEventNextId
	te := AeTimeEvent{
		id:       id,
		mask:     mask,
		when:     GetMsTime() + interval,
		interval: interval,
		proc:     proc,
		extra:    extra,
		next:     loop.TimeEvents,
	}
	loop.timeEventNextId++
	loop.TimeEvents = &te
	return id
}

func GetMsTime() int64 {
	return time.Now().UnixNano() / 1e6
}

func (loop *AeLoop) RemoveTimeEvent(id int) {
	p := loop.TimeEvents
	var pre *AeTimeEvent
	for p != nil {
		if p.id == id {
			if pre == nil {
				loop.TimeEvents = p.next
			} else {
				pre.next = p.next
			}
			p.next = nil
			break
		}
		pre = p
		p = p.next
	}
}

// 用于创建一个epoll实例，用于在Linux上进行I/O事件通知
func AeLoopCreate() (*AeLoop, error) {
	epollFd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &AeLoop{
		FileEvents:      make(map[int]*AeFileEvent),
		fileEventFd:     epollFd,
		timeEventNextId: 1,
		stop:            false,
	}, nil
}

func (loop *AeLoop) AeWait() (tes []*AeTimeEvent, fes []*AeFileEvent) {
	timeout := loop.nearestTime() - GetMsTime()
	// 最近事件已经到了
	if timeout <= 0 {
		timeout = 10
	}
	var events [128]unix.EpollEvent
	n, err := unix.EpollWait(loop.fileEventFd, events[:], int(timeout))
	if err != nil {
		log.Printf("epoll wait warning: %v\n", err)
	}
	if n > 0 {
		log.Printf("ae get %v epoll events\n", n)
	}
	// 收集文件事件
	for i := 0; i < n; i++ {
		// epollin，可读事件
		if events[i].Events&unix.EPOLLIN != 0 {
			fe := loop.FileEvents[getFeKey(int(events[i].Fd), AE_READABLE)]
			if fe != nil {
				fes = append(fes, fe)
			}
		}
		// epollout，可写事件
		if events[i].Events&unix.EPOLLOUT != 0 {
			fe := loop.FileEvents[getFeKey(int(events[i].Fd), AE_WRITABLE)]
			if fe != nil {
				fes = append(fes, fe)
			}
		}
	}
	// 收集时间事件
	now := GetMsTime()
	p := loop.TimeEvents
	for p != nil {
		if p.when <= now {
			tes = append(tes, p)
		}
		p = p.next
	}
	return
}

func (loop *AeLoop) nearestTime() int64 {
	var nearest int64 = GetMsTime() + 1000
	p := loop.TimeEvents
	for p != nil {
		if p.when < nearest {
			nearest = p.when
		}
		p = p.next
	}
	return nearest
}

func (loop *AeLoop) AeProcess(tes []*AeTimeEvent, fes []*AeFileEvent) {
	for _, te := range tes {
		te.proc(loop, te.id, te.extra)
		if te.mask == AE_ONCE {
			loop.RemoveTimeEvent(te.id)
		} else {
			te.when = GetMsTime() + te.interval
		}
	}
	if len(fes) > 0 {
		log.Printf("ae is processing file events")
		for _, fe := range fes {
			fe.proc(loop, fe.fd, fe.extra)
		}
	}
}

func (loop *AeLoop) AeMain() {
	for !loop.stop {
		tes, fes := loop.AeWait()
		loop.AeProcess(tes, fes)
	}
}
