package btreestore

import (
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPagerWriteAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), FilePrefix+".btree")
	pager, _ := newPager(path)
	node := newLeafNode()
	ok, err := node.insertLeaf("key1", "value1")
	if err != nil || ok != true {
		t.Errorf("error inserting key: %v, ok: %v", err, ok)
	}
	encode, err := node.encodeLeaf()
	if err != nil {
		t.Errorf("error encoding node: %v", err)
	}
	pageID := pager.allocatePage()
	err = pager.writePage(encode, pageID)
	if err != nil {
		t.Errorf("error writing page: %v", err)
	}

	buff, err := pager.readPage(pageID)
	if err != nil {
		t.Errorf("error reading page: %v", err)
	}

	decodedNode, err := decodeLeaf(buff)
	if err != nil {
		t.Errorf("error decoding node: %v", err)
	}
	index, ok := decodedNode.searchLeaf("key1")
	if ok != true {
		t.Errorf("error searching key1: %v", ok)
	}
	if index != 0 {
		t.Errorf("error searching key1, expected index 0 but found: %v", index)
	}
	if decodedNode.kv[index].value != "value1" {
		t.Errorf("error searching key1, expected value1 but found: %v", index)
	}
}

func TestAllocatePageSequencing(t *testing.T) {
	path := filepath.Join(t.TempDir(), FilePrefix+".btree")
	pager, _ := newPager(path)
	pageID1 := pager.allocatePage()
	pageID2 := pager.allocatePage()
	pageID3 := pager.allocatePage()
	pageID4 := pager.allocatePage()

	if pageID1 != 0 || pageID2 != 1 || pageID3 != 2 || pageID4 != 3 {
		t.Errorf("Sequencing data is not correct it should be 0, 1, 2, 3 but found %d, %d, %d, %d", pageID1, pageID2, pageID3, pageID4)
	}

}

func TestReadPageInvalidPageID(t *testing.T) {
	path := filepath.Join(t.TempDir(), FilePrefix+".btree")
	pager, _ := newPager(path)
	_, err := pager.readPage(0)
	if !errors.Is(err, InvalidPageIDError) {
		t.Errorf("should have thrown error as page id is invalid %d", 0)
	}
	pager.allocatePage()
	_, err = pager.readPage(1)
	if !errors.Is(err, InvalidPageIDError) {
		t.Errorf("should have thrown error as page id is invalid %d", 0)
	}
	// page allocated but not written and trying to read empty file
	_, err = pager.readPage(0)
	if !errors.Is(err, io.EOF) {
		t.Errorf("should have thrown io.EOF as page id is invalid %d", 0)
	}
}

func TestWritePageWithWrongBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), FilePrefix+".btree")
	pager, _ := newPager(path)
	pageID1 := pager.allocatePage()
	buff := make([]byte, 100)
	err := pager.writePage(buff, pageID1)
	if !errors.Is(err, PageSizeMismatchError) {
		t.Errorf("should have thrown error as wrong size page trying to save, page size %d, expected size %d", len(buff), pageSize)
	}
}

func TestPagerPersistedAcrossMultipleInstance(t *testing.T) {
	node1 := newLeafNode()
	node2 := newLeafNode()
	node3 := newLeafNode()

	_, err := node1.insertLeaf("key1", "value1")
	if err != nil {
		t.Errorf("error inserting key: %v", err)
	}
	_, err = node2.insertLeaf("key2", "value2")
	if err != nil {
		t.Errorf("error inserting key: %v", err)
	}
	_, err = node3.insertLeaf("key3", "value3")
	if err != nil {
		t.Errorf("error inserting key: %v", err)
	}
	path := filepath.Join(t.TempDir(), FilePrefix+".btree")

	pagerInstance1, _ := newPager(path)

	pageID1 := pagerInstance1.allocatePage()
	encode1, err := node1.encodeLeaf()
	if err != nil {
		t.Errorf("error encoding node: %v", err)
	}
	err = pagerInstance1.writePage(encode1, pageID1)
	if err != nil {
		t.Errorf("error writing page: %v", err)
	}

	pageID2 := pagerInstance1.allocatePage()
	encode2, err := node2.encodeLeaf()
	if err != nil {
		t.Errorf("error encoding node: %v", err)
	}
	err = pagerInstance1.writePage(encode2, pageID2)
	if err != nil {
		t.Errorf("error writing page: %v", err)
	}

	pageID3 := pagerInstance1.allocatePage()
	encode3, err := node3.encodeLeaf()
	if err != nil {
		t.Errorf("error encoding node: %v", err)
	}
	err = pagerInstance1.writePage(encode3, pageID3)
	if err != nil {
		t.Errorf("error writing page: %v", err)
	}
	err = pagerInstance1.Sync()
	if err != nil {
		t.Errorf("error syncing page: %v", err)
	}

	err = pagerInstance1.Close()
	if err != nil {
		t.Errorf("error closing page: %v", err)
	}

	pagerInstance2, _ := newPager(path)
	page1Encoded, err := pagerInstance2.readPage(pageID1)
	if err != nil {
		t.Errorf("error reading page: %v", err)
	}

	if !reflect.DeepEqual(page1Encoded, encode1) {
		t.Errorf("error reading page, expected %v but found %v", encode1, page1Encoded)
	}

	page2Encoded, err := pagerInstance2.readPage(pageID2)
	if err != nil {
		t.Errorf("error reading page: %v", err)
	}

	if !reflect.DeepEqual(page2Encoded, encode2) {
		t.Errorf("error reading page, expected %v but found %v", encode2, page2Encoded)
	}

	page3Encoded, err := pagerInstance2.readPage(pageID3)
	if err != nil {
		t.Errorf("error reading page: %v", err)
	}

	if !reflect.DeepEqual(page3Encoded, encode3) {
		t.Errorf("error reading page, expected %v but found %v", encode3, page3Encoded)
	}

	if pagerInstance2.pageCounter != 3 {
		t.Errorf("pagerInstance2.pageCounter should be 3 but found %d", pagerInstance2.pageCounter)
	}

	err = pagerInstance2.Close()
	if err != nil {
		t.Errorf("error closing page: %v", err)
	}

}
