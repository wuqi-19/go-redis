package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestList(t *testing.T) {
	list := ListCreate(ListType{EqualFunc: GStrEqual})
	assert.Equal(t, list.Length(), 0)

	list.Append(CreateObject(GSTR, "4"))
	// 4
	list.DelNode(list.First())
	//

	list.Append(CreateObject(GSTR, "1"))
	// 1
	list.Append(CreateObject(GSTR, "2"))
	// 1 2
	list.Append(CreateObject(GSTR, "3"))
	// 1 2 3
	assert.Equal(t, list.Length(), 3)
	assert.Equal(t, list.First().Val.Val_.(string), "1")
	assert.Equal(t, list.Last().Val.Val_.(string), "3")

	o := CreateObject(GSTR, "0")
	list.LPush(o)
	// 0 1 2 3
	assert.Equal(t, list.Length(), 4)
	assert.Equal(t, list.First().Val.Val_.(string), "0")

	list.LPush(CreateObject(GSTR, "-1"))
	// -1 0 1 2 3
	assert.Equal(t, list.Length(), 5)

	n := list.Find(o)
	assert.Equal(t, n.Val, o)

	list.Delete(o)
	// -1 1 2 3
	assert.Equal(t, list.Length(), 4)
	n = list.Find(o)
	assert.Nil(t, n)

	list.DelNode(list.First())
	// 1 2 3
	assert.Equal(t, list.Length(), 3)
	assert.Equal(t, list.First().Val.Val_.(string), "1")

	list.DelNode(list.Last())
	// 1 2
	assert.Equal(t, list.Length(), 2)
	assert.Equal(t, list.Last().Val.Val_.(string), "2")
}
