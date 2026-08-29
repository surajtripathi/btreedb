package btreestore

import (
	"encoding/binary"
	"fmt"
	"sort"
)

var ErrorOverFlow = fmt.Errorf("overflow error")

type keyValue struct {
	key   string
	value string
}

/*
	page layout
	page type 8 bits + key value count 16 bits + free space offset value 16 bits + next page id 64 bits
	+ slot entries (key value count * 16 bits) + free space + key value data
	Leaf page header layout (total 13 bytes):
	  page[0:1]    page type          (1 byte,  uint8)
	  page[1:3]    key count          (2 bytes, uint16)
	  page[3:5]    free space offset  (2 bytes, uint16)
	  page[5:13]   next page ID       (8 bytes, uint64)

	Slot directory:
	  page[13 : 13+2*keyCount]   slot entries, 2 bytes each (uint16 offsets)

	Free space:
	  the gap between end of slot directory and start of cell data

	Cell data region (grows backward from end of page):
	  page[?:4096]   key/value cells

*/

const (
	pageSize          = 4 * 1024 // 4 KB
	pageTypeStart     = 0        // 1 Byte
	keyCountStart     = 1        // 2 Byte
	freePageStart     = 3        // 2 Byte
	nextPageIdStart   = 5        // 8 Byte
	slotDirStart      = 13
	pageEnd           = pageSize
	kvLengthStoreSize = 2
)

type NodeType byte

const LeafNodeType NodeType = 1
const InternalNodeType NodeType = 2
const SuperBlockNodeType NodeType = 3

func nodeType(page []byte) NodeType {
	return NodeType(page[pageTypeStart])
}

type LeafNode struct {
	kv             []keyValue
	nextPageID     uint64
	slotOffset     uint16
	freePageOffset uint16
}

const noNextPage = 0

func newLeafNode() *LeafNode {
	return &LeafNode{
		kv:             make([]keyValue, 0),
		nextPageID:     noNextPage,
		slotOffset:     slotDirStart,
		freePageOffset: pageEnd,
	}
}

/*
A single key/value larger than ~half a page can cause a split to still overflow,
or fail to fit at all even in an empty leaf.
Not handled — encode() will surface ErrorOverFlow rather than corrupt data,
but Put won't give a clean 'key too large' message in this case.
*/
func (ln *LeafNode) encodeLeaf() ([]byte, error) {
	page := make([]byte, pageSize)

	page[pageTypeStart] = byte(LeafNodeType)

	binary.BigEndian.PutUint16(page[keyCountStart:freePageStart], uint16(len(ln.kv)))

	// page[3:5] = free page offset, comes later

	binary.BigEndian.PutUint64(page[nextPageIdStart:slotDirStart], ln.nextPageID)

	slotPointer := slotDirStart
	freePagePointer := pageEnd
	for _, kv := range ln.kv {
		keyValBuff := encodeKeyValueLeaf(kv.key, kv.value)

		keyValueStartingOffset := freePagePointer - len(keyValBuff)

		if keyValueStartingOffset < (slotPointer + 2) {

			return nil, ErrorOverFlow
		}

		copy(page[keyValueStartingOffset:freePagePointer], keyValBuff)
		binary.BigEndian.PutUint16(page[slotPointer:slotPointer+2], uint16(keyValueStartingOffset))

		slotPointer += 2
		freePagePointer = keyValueStartingOffset
	}

	binary.BigEndian.PutUint16(page[freePageStart:nextPageIdStart], uint16(freePagePointer))

	return page, nil
}

func (ln *LeafNode) searchLeaf(key string) (int, bool) {
	index := sort.Search(len(ln.kv), func(i int) bool {
		return ln.kv[i].key >= key
	})
	return index, index < len(ln.kv) && ln.kv[index].key == key
}

func (ln *LeafNode) insertLeaf(key string, value string) (bool, error) {
	index, ok := ln.searchLeaf(key)
	pageFreeSpace := int(ln.freePageOffset) - int(ln.slotOffset)
	if ok {
		additionalSpace := len(value) - len(ln.kv[index].value)
		if additionalSpace > pageFreeSpace {
			return false, ErrorOverFlow
		}
		newFreePageOffset := int(ln.freePageOffset) - additionalSpace
		ln.freePageOffset = uint16(newFreePageOffset)
		ln.kv[index].value = value
		return true, nil
	}
	requiredSpace := len(key) + len(value) + kvLengthStoreSize + kvLengthStoreSize + 2

	if requiredSpace > pageFreeSpace {
		return false, ErrorOverFlow
	}
	oldKv := ln.kv
	kv := make([]keyValue, len(oldKv)+1)
	copy(kv[0:index], oldKv[0:index])
	kv[index] = keyValue{key, value}
	copy(kv[index+1:], oldKv[index:])
	ln.kv = kv
	ln.freePageOffset = ln.freePageOffset - uint16(requiredSpace-2)
	ln.slotOffset = ln.slotOffset + 2

	return true, nil
}

func (ln *LeafNode) splitLeaf(neyKey string, newValue string) (*LeafNode, *LeafNode, string, error) {
	index, ok := ln.searchLeaf(neyKey)
	tempLen := len(ln.kv)
	if !ok {
		tempLen += 1
	}
	tempKV := make([]keyValue, tempLen)

	if ok {
		copy(tempKV[0:tempLen], ln.kv[0:tempLen])
		tempKV[index].value = newValue
	} else {
		copy(tempKV[0:index], ln.kv[0:index])
		tempKV[index] = keyValue{neyKey, newValue}
		copy(tempKV[index+1:], ln.kv[index:])
	}

	i := 0
	leftNode := newLeafNode()
	rightNode := newLeafNode()
	for ; i < len(tempKV)/2; i++ {
		ok, err := leftNode.insertLeaf(tempKV[i].key, tempKV[i].value)
		if !ok {
			return nil, nil, "", err
		}
	}
	promotedKey := tempKV[i].key
	for ; i < len(tempKV); i++ {
		ok, err := rightNode.insertLeaf(tempKV[i].key, tempKV[i].value)
		if !ok {
			return nil, nil, "", err
		}
	}
	return leftNode, rightNode, promotedKey, nil
}

func decodeLeaf(page []byte) (*LeafNode, error) {

	//pageType := byte(page[pageTypeStart])
	keyCount := binary.BigEndian.Uint16(page[keyCountStart:freePageStart])
	nextPageId := binary.BigEndian.Uint64(page[nextPageIdStart:slotDirStart])

	node := LeafNode{nextPageID: nextPageId, kv: make([]keyValue, keyCount)}
	node.freePageOffset = binary.BigEndian.Uint16(page[freePageStart:nextPageIdStart])
	slotPointer := slotDirStart
	for i := 0; i < int(keyCount); i++ {
		kvOffset := binary.BigEndian.Uint16(page[slotPointer : slotPointer+2])
		kv := decodeKeyValueLeaf(page, kvOffset)
		node.kv[i] = kv
		slotPointer += 2
	}
	node.slotOffset = uint16(slotPointer)
	return &node, nil
}

/*
keyLen(2) + key + valLen(2) + val
*/

func encodeKeyValueLeaf(key string, value string) []byte {

	buffer := make([]byte, kvLengthStoreSize+len(key)+kvLengthStoreSize+len(value))
	offset := 0
	binary.BigEndian.PutUint16(buffer[offset:offset+kvLengthStoreSize], uint16(len(key)))
	offset += kvLengthStoreSize

	copy(buffer[offset:offset+len(key)], key)
	offset += len(key)

	binary.BigEndian.PutUint16(buffer[offset:offset+kvLengthStoreSize], uint16(len(value)))
	offset += kvLengthStoreSize

	copy(buffer[offset:offset+len(value)], value)
	return buffer
}

func decodeKeyValueLeaf(page []byte, offset uint16) keyValue {
	kvOff := offset
	keyLen := binary.BigEndian.Uint16(page[kvOff : kvOff+kvLengthStoreSize])
	kvOff += kvLengthStoreSize
	key := string(page[kvOff : kvOff+keyLen])
	kvOff += keyLen

	valueLen := binary.BigEndian.Uint16(page[kvOff : kvOff+kvLengthStoreSize])
	kvOff += kvLengthStoreSize
	value := string(page[kvOff : kvOff+valueLen])

	return keyValue{key, value}
}
