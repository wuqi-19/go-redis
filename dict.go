package main

import (
	"errors"
	"math"
	"math/rand"
)

const (
	DEFAULT_STEP int   = 1
	INIT_SIZE    int64 = 8
	FORCE_RATIO  int64 = 2
	GROW_RATIO   int64 = 2
)

var (
	EP_ERR = errors.New("expand error")
	EX_ERR = errors.New("key exists error")
	NK_ERR = errors.New("key doesnt exist error")
)

type Dict struct {
	DictType
	hts       [2]*htable
	rehashidx int64
}

type DictType struct {
	HashFunc  func(key *Gobj) int64
	EqualFunc func(k1, k2 *Gobj) bool
}

type htable struct {
	table []*Entry
	size  int64
	mask  int64 // 掩码
	used  int64
}

type Entry struct {
	Key  *Gobj
	Val  *Gobj
	next *Entry
}

func DictCreate(dictType DictType) *Dict {
	return &Dict{
		DictType:  dictType,
		rehashidx: -1,
	}
}

func (dict *Dict) Get(key *Gobj) *Gobj {
	entry := dict.Find(key)
	if entry == nil {
		return nil
	}
	return entry.Val
}

// 随机选择，可能是用于删除之类的
func (dict *Dict) RandomGet() *Entry {
	if dict.hts[0] == nil {
		return nil
	}
	t := 0
	if dict.isRehashing() {
		dict.rehashStep()
		if dict.hts[1] != nil && dict.hts[1].used > dict.hts[1].used {
			t = 1
		}
	}
	// 获取随机的table结点
	idx := rand.Int63n(dict.hts[t].size)
	cnt := 0
	for dict.hts[t].table[idx] == nil && cnt < 1000 {
		idx = rand.Int63n(dict.hts[t].size)
		cnt += 1
	}
	if dict.hts[t].table[idx] == nil {
		return nil
	}
	// 获取该结点里随机的元素
	var listLen int64
	p := dict.hts[t].table[idx]
	for p != nil {
		listLen += 1
		p = p.next
	}
	listIdx := rand.Int63n(listLen)
	p = dict.hts[t].table[idx]
	for i := int64(0); i < listIdx; i++ {
		p = p.next
	}
	return p
}

func (dict *Dict) Set(key, val *Gobj) {
	if err := dict.Add(key, val); err == nil {
		return
	}
	entry := dict.Find(key)
	entry.Val.DecrRefCount()
	entry.Val = val
	val.IncrRefCount()
}

func (dict *Dict) Add(key, val *Gobj) error {
	entry := dict.AddRaw(key)
	if entry == nil {
		return EX_ERR
	}
	entry.Val = val
	val.IncrRefCount()
	return nil
}

// 每次查找，都rehash一步
func (dict *Dict) Find(key *Gobj) *Entry {
	if dict.hts[0] == nil {
		return nil
	}
	if dict.isRehashing() {
		dict.rehashStep()
	}
	// 在两个表中寻找
	h := dict.HashFunc(key)
	for i := 0; i <= 1; i++ {
		idx := h & dict.hts[i].mask
		e := dict.hts[i].table[idx]
		for e != nil {
			if dict.EqualFunc(e.Key, key) {
				return e
			}
			e = e.next
		}
		if !dict.isRehashing() {
			break
		}
	}
	return nil
}

// 每次删除，都rehash一步
func (dict *Dict) Delete(key *Gobj) error {
	if dict.hts[0] == nil {
		return NK_ERR
	}
	if dict.isRehashing() {
		dict.rehashStep()
	}
	h := dict.HashFunc(key)
	for i := 0; i <= 1; i++ {
		idx := h & dict.hts[i].mask
		e := dict.hts[i].table[idx]
		var prev *Entry
		for e != nil {
			if dict.EqualFunc(e.Key, key) {
				if prev == nil {
					dict.hts[i].table[idx] = e.next
				} else {
					prev.next = e.next
				}
				freeEntry(e)
				return nil
			}
			prev = e
			e = e.next
		}
		if !dict.isRehashing() {
			break
		}
	}
	return NK_ERR
}

func freeEntry(e *Entry) {
	e.Key.DecrRefCount()
	e.Val.DecrRefCount()
}

// 每次AddRaw，都rehash一步
func (dict *Dict) AddRaw(key *Gobj) *Entry {
	if dict.isRehashing() {
		dict.rehashStep()
	}
	idx := dict.keyIndex(key)
	if idx == -1 {
		return nil
	}

	var ht *htable
	if dict.isRehashing() {
		ht = dict.hts[1]
	} else {
		ht = dict.hts[0]
	}
	e := Entry{
		Key:  key,
		next: ht.table[idx],
	}
	key.IncrRefCount()
	ht.table[idx] = &e
	ht.used += 1
	return &e
}

// 判断是否需要扩容，以及检验key是否存在
func (dict *Dict) keyIndex(key *Gobj) int64 {
	err := dict.expandIfNeeded()
	if err != nil {
		return -1
	}
	h := dict.HashFunc(key)
	var idx int64
	for i := 0; i <= 1; i++ {
		idx = h & dict.hts[i].mask
		// 校验key是否已存在
		e := dict.hts[i].table[idx]
		for e != nil {
			if dict.EqualFunc(e.Key, key) {
				return -1
			}
			e = e.next
		}
		// rehash过程中，会返回第二个table的idx
		if !dict.isRehashing() {
			break
		}
	}
	return idx
}

func (dict *Dict) expandIfNeeded() error {
	if dict.isRehashing() {
		return nil
	}
	if dict.hts[0] == nil {
		return dict.expand(INIT_SIZE)
	}
	if (dict.hts[0].used > dict.hts[0].size) && dict.hts[0].used/dict.hts[0].size > FORCE_RATIO {
		return dict.expand(dict.hts[0].size * GROW_RATIO)
	}
	return nil
}

func (dict *Dict) expand(size int64) error {
	sz := nextPower(size)
	if dict.isRehashing() || (dict.hts[0] != nil && dict.hts[0].size >= sz) {
		return EP_ERR
	}
	ht := htable{
		table: make([]*Entry, sz),
		size:  sz,
		mask:  sz - 1,
		used:  0,
	}
	if dict.hts[0] == nil {
		dict.hts[0] = &ht
		return nil
	}
	dict.hts[1] = &ht
	dict.rehashidx = 0
	return nil
}

func nextPower(size int64) int64 {
	for i := INIT_SIZE; i < math.MaxInt64; i *= 2 {
		if i >= size {
			return i
		}
	}
	return -1
}

func (dict *Dict) isRehashing() bool {
	return dict.rehashidx != -1
}

func (dict *Dict) rehashStep() {
	//TODO: check iterators
	dict.rehash(DEFAULT_STEP)
}

func (dict *Dict) rehash(step int) {
	for step > 0 {
		if dict.hts[0].used == 0 {
			dict.hts[0] = dict.hts[1]
			dict.hts[1] = nil
			dict.rehashidx = -1
			return
		}

		for dict.hts[0].table[dict.rehashidx] == nil {
			dict.rehashidx += 1
		}

		entry := dict.hts[0].table[dict.rehashidx]
		for entry != nil {
			ne := entry.next
			idx := dict.HashFunc(entry.Key) & dict.hts[1].mask
			// 头插法
			entry.next = dict.hts[1].table[idx]
			dict.hts[1].table[idx] = entry
			dict.hts[0].used -= 1
			dict.hts[1].used += 1
			entry = ne
		}
		dict.hts[0].table[dict.rehashidx] = nil
		dict.rehashidx += 1
		step -= 1
	}
}
