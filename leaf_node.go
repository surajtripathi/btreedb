package btreestore

import (
	"encoding/binary"
	"fmt"
)

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

type LeafNode struct {
	kv         []keyValue
	nextPageID uint64
}

func (ln *LeafNode) encode() ([]byte, error) {
	page := make([]byte, pageSize)

	page[pageTypeStart] = byte(LeafNodeType)

	binary.BigEndian.PutUint16(page[keyCountStart:freePageStart], uint16(len(ln.kv)))

	// page[3:5] = free page offset, comes later

	binary.BigEndian.PutUint64(page[nextPageIdStart:slotDirStart], ln.nextPageID)

	slotPointer := slotDirStart
	freePagePointer := pageEnd
	for _, kv := range ln.kv {
		keyValBuff := encodeKeyValue(kv.key, kv.value)

		keyValueStartingOffset := freePagePointer - len(keyValBuff)

		if keyValueStartingOffset < (slotPointer + 2) {
			return nil, fmt.Errorf("overflow error")
		}

		copy(page[keyValueStartingOffset:freePagePointer], keyValBuff)
		binary.BigEndian.PutUint16(page[slotPointer:slotPointer+2], uint16(keyValueStartingOffset))

		slotPointer += 2
		freePagePointer = keyValueStartingOffset
	}

	binary.BigEndian.PutUint16(page[freePageStart:nextPageIdStart], uint16(freePagePointer))
	return page, nil
}

func decode(page []byte) (*LeafNode, error) {

	//pageType := byte(page[pageTypeStart])
	keyCount := binary.BigEndian.Uint16(page[keyCountStart:freePageStart])
	// page[3:5] = free page offset, comes later
	nextPageId := binary.BigEndian.Uint64(page[nextPageIdStart:slotDirStart])

	node := LeafNode{nextPageID: nextPageId, kv: make([]keyValue, keyCount)}
	slotPointer := slotDirStart
	for i := 0; i < int(keyCount); i++ {
		kvOffset := binary.BigEndian.Uint16(page[slotPointer : slotPointer+2])
		kv := decodeKeyValue(page, kvOffset)
		node.kv[i] = kv
		slotPointer += 2
	}
	return &node, nil
}

/*
keyLen(2) + key + valLen(2) + val
*/

func encodeKeyValue(key string, value string) []byte {

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

func decodeKeyValue(page []byte, offset uint16) keyValue {
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
