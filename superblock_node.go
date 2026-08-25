package btreestore

import (
	"encoding/binary"
	"errors"
)

type SuperBlockNode struct {
	rootPageID uint64
}

func newSuperBlockNode(rootPageID uint64) *SuperBlockNode {
	return &SuperBlockNode{
		rootPageID: rootPageID,
	}
}

func (s *SuperBlockNode) encodeSuperBlock() ([]byte, error) {
	buff := make([]byte, pageSize)
	offset := 0
	buff[offset] = byte(SuperBlockNodeType)
	offset++
	binary.BigEndian.PutUint64(buff[offset:offset+8], s.rootPageID)
	offset += 8
	return buff, nil
}
func decodeSuperBlock(buff []byte) (*SuperBlockNode, error) {
	offset := 0
	pageType := NodeType(buff[offset])
	if pageType != SuperBlockNodeType {
		return nil, errors.New("wrong node type")
	}
	offset++
	rootPageID := binary.BigEndian.Uint64(buff[offset : offset+8])
	offset += 8
	return &SuperBlockNode{
		rootPageID: rootPageID,
	}, nil
}
