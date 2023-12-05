package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDict(t *testing.T) {
	dict := DictCreate(DictType{HashFunc: GStrHash, EqualFunc: GStrEqual})
	entry := dict.RandomGet()
	assert.Nil(t, entry)

	k1 := CreateObject(GSTR, "k1")
	v1 := CreateObject(GSTR, "v1")
	e := dict.Add(k1, v1)
	assert.Nil(t, e)

	entry = dict.Find(k1)
	assert.Equal(t, k1, entry.Key)
	assert.Equal(t, v1, entry.Val)
	assert.Equal(t, 2, k1.refCount)
	assert.Equal(t, 2, v1.refCount)

	e = dict.Delete(k1)
	assert.Nil(t, e)
	entry = dict.Find(k1)
	assert.Nil(t, entry)
	assert.Equal(t, 1, k1.refCount)
	assert.Equal(t, 1, v1.refCount)

	e = dict.Add(k1, v1)
	assert.Nil(t, e)
	v := dict.Get(k1)
	assert.Equal(t, v1, v)
	v2 := CreateObject(GSTR, "v2")
	dict.Set(k1, v2)
	v = dict.Get(k1)
	assert.Equal(t, v2, v)
	assert.Equal(t, 2, v2.refCount)
	assert.Equal(t, 1, v1.refCount)
}

func TestRehash(t *testing.T) {
	dict := DictCreate(DictType{HashFunc: GStrHash, EqualFunc: GStrEqual})
	entry := dict.RandomGet()
	assert.Nil(t, entry)

	// dict.hts[0].used/dict.hts[0].size > FORCE_RATIO
	value := int(INIT_SIZE * (FORCE_RATIO + 1)) // 24
	for i := 0; i < value; i++ {
		key := CreateObject(GSTR, fmt.Sprintf("k%v", i))
		val := CreateObject(GSTR, fmt.Sprintf("v%v", i))
		// 会先判断是否需要扩容，再add
		// 判断扩容时，used = 23
		e := dict.Add(key, val)
		assert.Nil(t, e)
	}
	assert.Equal(t, false, dict.isRehashing())

	key := CreateObject(GSTR, fmt.Sprintf("k%v", value))
	val := CreateObject(GSTR, fmt.Sprintf("v%v", value))
	e := dict.Add(key, val)
	assert.Nil(t, e)
	assert.Equal(t, true, dict.isRehashing())
	assert.Equal(t, int64(0), dict.rehashidx)
	assert.Equal(t, int64(INIT_SIZE), dict.hts[0].size)
	assert.Equal(t, int64(INIT_SIZE*GROW_RATIO), dict.hts[1].size)

	for i := 0; i <= int(INIT_SIZE); i++ {
		dict.RandomGet()
	}
	assert.Equal(t, false, dict.isRehashing())
	assert.Equal(t, int64(INIT_SIZE*GROW_RATIO), dict.hts[0].size)
	assert.Nil(t, dict.hts[1])

	for i := 0; i <= value; i++ {
		key := CreateObject(GSTR, fmt.Sprintf("k%v", i))
		entry = dict.Find(key)
		assert.NotNil(t, entry)
		assert.Equal(t, fmt.Sprintf("v%v", i), entry.Val.StrVal())
	}
}
