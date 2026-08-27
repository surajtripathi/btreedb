package btreestore

import (
	"path/filepath"
	"testing"
)

func TestSuperBlockEncodeDecode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "btree.db")
	walPath := filepath.Join(dir, "test.wal")
	store, err := Open(dbPath, walPath)
	if err != nil {
		t.Fatal(err)
	}

	superBlockPage, err := store.pager.readPage(superBlockPageID)
	if err != nil {
		t.Fatal(err)
	}

	if nodeType(superBlockPage) != SuperBlockNodeType {
		t.Fatalf("Expected superBlockNode to be of type SuperBlockNode but got %v", nodeType(superBlockPage))
	}
	sbNode, err := decodeSuperBlock(superBlockPage)
	if err != nil {
		t.Fatal(err)
	}
	if sbNode.rootPageID != store.superBlock.rootPageID {
		t.Fatalf("Expected superBlockNode to have root page ID %v but got %v", store.superBlock.rootPageID, sbNode.rootPageID)
	}

	sbNode.rootPageID = 1000

	superBlockEncoded, err := sbNode.encodeSuperBlock()
	if err != nil {
		t.Fatal(err)
	}
	err = store.pager.writePage(superBlockEncoded, superBlockPageID)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Close()
	if err != nil {
		t.Fatal(err)
	}
	store, err = Open(dbPath, walPath)
	if err != nil {
		t.Fatal(err)
	}
	if store.superBlock.rootPageID != 1000 {
		t.Fatalf("root page id was supposed to be updated to 1000 but got %v", store.superBlock.rootPageID)
	}
}
