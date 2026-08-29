package btreestore

import (
	"errors"
	"fmt"
)

type Store struct {
	pager         *Pager
	superBlock    *SuperBlockNode
	wal           *Wal
	checkPointing *CheckPointing
}

type StoreOptions struct {
	WalPath                string
	DBPath                 string
	CheckpointingThreshold int
}

const superBlockPageID = 0

var NotFound = errors.New("not found")

func Open(so StoreOptions) (*Store, error) {
	pager, err := newPager(so.DBPath)
	if err != nil {
		return nil, err
	}
	store := &Store{
		pager: pager,
	}

	if store.pager.pageCounter == 0 {
		// no file exists
		// superBlockPageID should be always zero
		allocatedSuperBlockPageID := store.pager.allocatePage()

		if allocatedSuperBlockPageID != 0 {
			return nil, fmt.Errorf("superBlockPageID already allocated: %d", allocatedSuperBlockPageID)
		}

		rootPageID := store.pager.allocatePage()
		superBlockNode := newSuperBlockNode(rootPageID)
		store.superBlock = superBlockNode

		encodedSuperBlock, err := superBlockNode.encodeSuperBlock()
		if err != nil {
			return nil, err
		}
		err = store.pager.writePage(encodedSuperBlock, allocatedSuperBlockPageID)
		if err != nil {
			return nil, err
		}

		emptyRootNode := newLeafNode()
		encode, err := emptyRootNode.encodeLeaf()
		if err != nil {
			return nil, err
		}

		err = store.pager.writePage(encode, superBlockNode.rootPageID)
		if err != nil {
			return nil, err
		}
	} else {
		superBlockPage, err := store.pager.readPage(superBlockPageID)
		if err != nil {
			return nil, err
		}
		superBlockNode, err := decodeSuperBlock(superBlockPage)
		if err != nil {
			return nil, err
		}
		store.superBlock = superBlockNode
	}
	err = WalReplay(so.WalPath, func(record WalRecord) error {
		switch record.op {
		case opPut:
			err := store.applyPut(record.key, record.value)
			if err != nil {
				return err
			}
		case opDelete:
			err := store.Delete(record.key)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown op: %d", record.op)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	wal, err := OpenWAL(so.WalPath)
	if err != nil {
		return nil, err
	}
	err = wal.Truncate()
	if err != nil {
		return nil, err
	}
	store.wal = wal
	store.checkPointing = newCheckPointing(so.CheckpointingThreshold)
	return store, nil
}

func (s *Store) Get(key string) (string, error) {
	node, err := s.findLeaf(key)
	if err != nil {
		return "", err
	}
	index, ok := node.searchLeaf(key)
	if !ok {
		return "", NotFound
	}
	return node.kv[index].value, nil
}

func (s *Store) findLeaf(key string) (*LeafNode, error) {
	pageID := s.superBlock.rootPageID
	for {
		page, err := s.pager.readPage(pageID)
		if err != nil {
			return nil, err
		}
		switch nodeType(page) {
		case InternalNodeType:
			internalNode, err := decodeInternal(page)
			if err != nil {
				return nil, err
			}
			index := internalNode.searchInternal(key)
			if index >= len(internalNode.children) {
				return nil, errors.New(fmt.Sprintf("index %d out of range", index))
			}
			pageID = internalNode.children[index]
		case LeafNodeType:
			node, err := decodeLeaf(page)
			if err != nil {
				return nil, err
			}

			return node, nil
		default:
			return nil, errors.New("invalid page")
		}
	}
}

func (s *Store) RangeGet(start string, end string) ([]keyValue, error) {
	node, err := s.findLeaf(start)
	if err != nil {
		return nil, err
	}
	index, _ := node.searchLeaf(start)

	data := make([]keyValue, 0)

	i := index
	for {
		if i < len(node.kv) && node.kv[i].key <= end {
			data = append(data, node.kv[i])
			i++
		} else if i == len(node.kv) && node.nextPageID != noNextPage {
			// read next page, zero is default page id
			// end page should be set it to zero
			nextPage, err := s.pager.readPage(node.nextPageID)
			if err != nil {
				return nil, err
			}
			node, err = decodeLeaf(nextPage)
			if err != nil {
				return nil, err
			}
			// reset i
			i = 0
		} else {
			return data, nil
		}
	}
}

func (s *Store) Put(key string, value string) error {
	err := s.wal.Append(WalRecord{op: opPut, key: key, value: value})
	if err != nil {
		return err
	}
	err = s.applyPut(key, value)
	if err != nil {
		return err
	}

	s.checkPointing.incr()
	if s.checkPointing.check() {
		err = s.doCheckPointing()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyPut(key string, value string) error {
	pageID := s.superBlock.rootPageID
	path := make([]uint64, 0)
	for {
		page, err := s.pager.readPage(pageID)
		if err != nil {
			return err
		}
		switch nodeType(page) {
		case InternalNodeType:
			internalNode, err := decodeInternal(page)
			if err != nil {
				return err
			}
			index := internalNode.searchInternal(key)
			if index >= len(internalNode.children) {
				return errors.New(fmt.Sprintf("index %d out of range", index))
			}
			path = append(path, pageID)
			pageID = internalNode.children[index]
		case LeafNodeType:
			node, err := decodeLeaf(page)
			if err != nil {
				return err
			}
			ok, err := node.insertLeaf(key, value)
			if ok {
				encodedLeaf, err := node.encodeLeaf()
				if err != nil {
					return err
				}
				err = s.pager.writePage(encodedLeaf, pageID)
				if err != nil {
					return err
				}
			} else if errors.Is(err, ErrorOverFlow) {
				l, r, pk, err := node.splitLeaf(key, value)
				if err != nil {
					return err
				}
				pkPageID := s.pager.allocatePage()
				r.nextPageID = node.nextPageID
				l.nextPageID = pkPageID
				rightEncoded, err := r.encodeLeaf()
				if err != nil {
					return err
				}
				err = s.pager.writePage(rightEncoded, pkPageID)
				if err != nil {
					return err
				}
				leftEncoded, err := l.encodeLeaf()
				if err != nil {
					return err
				}
				err = s.pager.writePage(leftEncoded, pageID)
				if err != nil {
					return err
				}
				err = s.propagateUpdateToPath(pageID, pkPageID, pk, path)
				if err != nil {
					return err
				}
			} else {
				return err
			}
			return nil
		default:
			return errors.New("invalid page")
		}

	}
}

func (s *Store) propagateUpdateToPath(leftPageID uint64, rightPageID uint64, propagationKey string, path []uint64) error {
	// new root
	if len(path) == 0 {
		pageID := s.pager.allocatePage()
		in, err := newInternalNode(leftPageID, rightPageID, propagationKey)
		if err != nil {
			if errors.Is(err, ErrorOverFlow) {
				return errors.New("key is so big that even the internal node creation is failing")
			}
			return err
		}
		internalEncoded, err := in.encodeInternal()
		if err != nil {
			return err
		}
		err = s.pager.writePage(internalEncoded, pageID)
		if err != nil {
			return err
		}
		s.superBlock.rootPageID = pageID
		encodedSuperBlock, err := s.superBlock.encodeSuperBlock()
		if err != nil {
			return err
		}
		err = s.pager.writePage(encodedSuperBlock, superBlockPageID)
		if err != nil {
			return err
		}
	} else {
		// update the existing internal node
		pageID := path[len(path)-1]
		page, err := s.pager.readPage(pageID)
		if err != nil {
			return err
		}
		in, err := decodeInternal(page)
		if err != nil {
			return err
		}
		err = in.insertInternal(propagationKey, rightPageID)
		if err != nil {
			if errors.Is(err, ErrorOverFlow) {
				//fmt.Printf("internal node split %v\n", in)
				left, right, promotedKey, err := in.splitInternal(propagationKey, rightPageID)
				//fmt.Printf("promoted key : %v\n", promotedKey)
				if err != nil {
					return err
				}
				newRightPageID := s.pager.allocatePage()
				leftEncoded, err := left.encodeInternal()
				if err != nil {
					return err
				}
				err = s.pager.writePage(leftEncoded, pageID)
				if err != nil {
					return err
				}
				rightEncoded, err := right.encodeInternal()
				if err != nil {
					return err
				}
				err = s.pager.writePage(rightEncoded, newRightPageID)
				if err != nil {
					return err
				}
				err = s.propagateUpdateToPath(pageID, newRightPageID, promotedKey, path[0:len(path)-1])
				if err != nil {
					return err
				}
				return nil
			}
			// need to implement recursive overflow of internal node that requires split implementation
			return err
		}
		encodedInternal, err := in.encodeInternal()
		if err != nil {
			return err
		}
		err = s.pager.writePage(encodedInternal, pageID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Delete(key string) error {
	err := s.wal.Append(WalRecord{op: opDelete, key: key})
	if err != nil {
		return err
	}
	err = s.applyDelete()
	if err != nil {
		return err
	}
	s.checkPointing.incr()
	if s.checkPointing.check() {
		err = s.doCheckPointing()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyDelete() error {
	return nil
}

func (s *Store) doCheckPointing() error {
	// sync
	err := s.pager.Sync()
	if err != nil {
		return err
	}
	// wal truncate
	err = s.wal.Truncate()
	if err != nil {
		return err
	}
	// reset counter
	s.checkPointing.reset()
	return nil
}

func (s *Store) Close() error {
	err := s.doCheckPointing()
	if err != nil {
		return err
	}
	err = s.pager.Close()
	if err != nil {
		return err
	}
	err = s.wal.Close()
	if err != nil {
		return err
	}
	return nil
}
