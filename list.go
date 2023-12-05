package main

type Node struct {
	Val  *Gobj
	next *Node
	prev *Node
}

type ListType struct {
	EqualFunc func(a, b *Gobj) bool
}

type List struct {
	ListType
	head   *Node
	tail   *Node
	length int
}

func ListCreate(ListType ListType) *List {
	var list List
	list.ListType = ListType
	return &list
}

func (list *List) Length() int {
	return list.length
}

func (list *List) First() *Node {
	return list.head
}

func (list *List) Last() *Node {
	return list.tail
}

func (list *List) Find(val *Gobj) *Node {
	p := list.head
	for p != nil {
		if list.EqualFunc(p.Val, val) {
			break
		}
		p = p.next
	}
	// 未找到返回nil
	return p
}

func (list *List) Append(val *Gobj) {
	// 尾插法
	var n Node
	n.Val = val
	// list为空
	if list.head == nil {
		list.head = &n
		list.tail = &n
	} else {
		n.prev = list.tail
		list.tail.next = &n
		list.tail = list.tail.next
	}
	list.length++
}

func (list *List) LPush(val *Gobj) {
	// 头插法
	var n Node
	n.Val = val
	// list为空
	if list.head == nil {
		list.head = &n
		list.tail = &n
	} else {
		n.next = list.head
		list.head.prev = &n
		list.head = &n
	}
	list.length++
}

func (list *List) DelNode(n *Node) {
	if n == nil {
		return
	}
	// 自定义结构，直接==比较的是引用内存地址
	if list.head == n {
		if n.next != nil {
			n.next.prev = nil
		}
		list.head = n.next
		n.next = nil
	} else if list.tail == n {
		if n.prev != nil {
			n.prev.next = nil
		}
		list.tail = n.prev
		n.prev = nil
	} else {
		if n.prev != nil {
			n.prev.next = n.next
		}
		if n.next != nil {
			n.next.prev = n.prev
		}
		n.prev = nil
		n.next = nil
	}
	list.length--
}

func (list *List) Delete(val *Gobj) {
	list.DelNode(list.Find(val))
}
