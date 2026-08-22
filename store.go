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
	page, err := s.pager.readPage(s.rootPageID)
	if err != nil {
		return err
	}
	node, err := decodeLeaf(page)
	if err != nil {
		return err
	}

	_, err = node.insertLeaf(key, value)
	if err != nil {
		return err
	}

	encodeNode, err := node.encodeLeaf()
	if err != nil {
		return err
	}

	err = s.pager.writePage(encodeNode, s.rootPageID)
	if err != nil {
		return err
	}
	err = s.pager.Sync()
	if err != nil {
		return err
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
