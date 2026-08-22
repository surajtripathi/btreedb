package btreestore

import (
	"encoding/binary"
	"sort"
)

type InternalNode struct {
	keys           []string // n keys
	children       []uint64 // n+1 children
	freePageOffset uint16
	slotOffset     uint16
}

/*
	page layout
	page type 8 bits + key count 16 bits + free space offset value 16 bits
	+ slot entries (key value count * 16 bits) + free space + key value data
	Leaf page header layout (total 13 bytes):
	  page[0:1]    page type          (1 byte,  uint8)
	  page[1:3]    key count          (2 bytes, uint16)
	  page[3:5]    free space offset  (2 bytes, uint16)

	Slot directory:
	  page[5 : 5+2*keyCount]   slot entries, 2 bytes each (uint16 offsets)

	Free space:
	  the gap between end of slot directory and start of cell data

	Cell data region (grows backward from end of page):
	  page[?:4096]   len(key)+key+pointers
      cell 0 is dummy key to handle children n+1 case
*/

const internalSlotDirStart = 5

func newInternalNode() *InternalNode {
	return &InternalNode{
		keys:     make([]string, 0),
		children: make([]uint64, 0),
	}
}

func (in *InternalNode) encodeInternal() ([]byte, error) {
	//if len(in.keys) != len(in.children) - 1 check needs to be implemented
	page := make([]byte, pageSize)

	page[pageTypeStart] = byte(InternalNodeType)

	binary.BigEndian.PutUint16(page[keyCountStart:freePageStart], uint16(len(in.keys)))

	// page[3:5] = free page offset, comes later

	slotPointer := internalSlotDirStart
	freePagePointer := pageEnd

	for idx, child := range in.children {
		key := ""
		if idx != 0 {
			key = in.keys[idx-1]
		}

		keyChildBuff := encodeKeyChildrenInternal(key, child)

		keyChildStartingOffset := freePagePointer - len(keyChildBuff)

		if keyChildStartingOffset < (slotPointer + 2) {

			return nil, ErrorOverFlow
		}

		copy(page[keyChildStartingOffset:freePagePointer], keyChildBuff)
		binary.BigEndian.PutUint16(page[slotPointer:slotPointer+2], uint16(keyChildStartingOffset))

		slotPointer += 2
		freePagePointer = keyChildStartingOffset
	}

	binary.BigEndian.PutUint16(page[freePageStart:internalSlotDirStart], uint16(freePagePointer))

	return page, nil
}

/*
item[0]: key=∅        , child=P0   (covers: everything < k1)
item[1]: key=k1        , child=P1   (covers: k1 <= x < k2)
item[2]: key=k2        , child=P2   (covers: k2 <= x < k3)
item[3]: key=k3        , child=P3   (covers: x >= k3)
*/

func (in *InternalNode) searchInternal(x string) int {
	index := sort.Search(len(in.keys), func(i int) bool {
		return in.keys[i] > x
	})
	return index
}

func decodeInternal(page []byte) (*InternalNode, error) {

	//pageType := byte(page[pageTypeStart])
	keyCount := binary.BigEndian.Uint16(page[keyCountStart:freePageStart])

	node := InternalNode{children: make([]uint64, keyCount+1), keys: make([]string, keyCount)}
	node.freePageOffset = binary.BigEndian.Uint16(page[freePageStart:internalSlotDirStart])
	slotPointer := internalSlotDirStart
	for i := 0; i < len(node.children); i++ {
		keyChildOffset := binary.BigEndian.Uint16(page[slotPointer : slotPointer+2])
		key, child := decodeKeyChildrenInternal(page, keyChildOffset)
		if i == 0 {
			node.children[i] = child
		} else {
			node.children[i] = child
			node.keys[i-1] = key
		}
		slotPointer += 2
	}
	node.slotOffset = uint16(slotPointer)
	return &node, nil
}

/*
keyLen(2) + key + child(8)
*/

func encodeKeyChildrenInternal(key string, pointer uint64) []byte {

	buffer := make([]byte, kvLengthStoreSize+len(key)+8)
	offset := 0
	binary.BigEndian.PutUint16(buffer[offset:offset+kvLengthStoreSize], uint16(len(key)))
	offset += kvLengthStoreSize

	copy(buffer[offset:offset+len(key)], key)
	offset += len(key)

	binary.BigEndian.PutUint64(buffer[offset:offset+8], pointer)
	offset += 8

	return buffer
}

func decodeKeyChildrenInternal(page []byte, offset uint16) (string, uint64) {
	keyChildOff := offset
	keyLen := binary.BigEndian.Uint16(page[keyChildOff : keyChildOff+kvLengthStoreSize])
	keyChildOff += kvLengthStoreSize
	key := string(page[keyChildOff : keyChildOff+keyLen])
	keyChildOff += keyLen

	child := binary.BigEndian.Uint64(page[keyChildOff : keyChildOff+8])
	keyChildOff += 8

	return key, child
}
