package btreestore

import (
	"errors"
	"fmt"
)

type Store struct {
	pager      *Pager
	rootPageID uint64
}

var NotFound = errors.New("not found")

func Open(dbPath string) (*Store, error) {
	pager, err := newPager(dbPath)
	if err != nil {
		return nil, err
	}
	store := &Store{
		pager:      pager,
		rootPageID: 0,
	}

	if store.pager.pageCounter == 0 {
		// no file exists
		rootPageID := store.pager.allocatePage()
		store.rootPageID = rootPageID

		emptyRootNode := newLeafNode()
		encode, err := emptyRootNode.encodeLeaf()
		if err != nil {
			return nil, err
		}

		err = store.pager.writePage(encode, rootPageID)
		if err != nil {
			return nil, err
		}
	} else {
		// hard coded for now
		store.rootPageID = 0
	}
	return store, nil
}

func (s *Store) Get(key string) (string, error) {
	pageID := s.rootPageID
	for {
		page, err := s.pager.readPage(pageID)
		if err != nil {
			return "", err
		}
		switch nodeType(page) {
		case InternalNodeType:
			internalNode, err := decodeInternal(page)
			if err != nil {
				return "", err
			}
			index := internalNode.searchInternal(key)
			if index >= len(internalNode.children) {
				return "", errors.New(fmt.Sprintf("index %d out of range", index))
			}
			pageID = internalNode.children[index]
		case LeafNodeType:
			node, err := decodeLeaf(page)
			if err != nil {
				return "", err
			}

			index, ok := node.searchLeaf(key)
			if !ok {
				return "", NotFound
			}
			return node.kv[index].value, nil
		default:
			return "", errors.New("invalid page")
		}
	}
}

func (s *Store) Put(key string, value string) error {
	pageID := s.rootPageID
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
			return s.pager.Sync()
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
		fmt.Printf("rootPage %v\n", pageID)
		fmt.Printf("root %v\n", in)
		s.rootPageID = pageID
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
				fmt.Printf("internal node split %v\n", in)
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
	return nil
}

func (s *Store) Close() error {
	err := s.pager.Close()
	if err != nil {
		return err
	}
	return nil
}
