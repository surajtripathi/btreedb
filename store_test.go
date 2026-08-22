package btreestore

import (
	"errors"
	"path/filepath"
	"strconv"
	"testing"
)

func createStoreWithData(path string) (*Store, error) {
	store, err := Open(path)
	if err != nil {
		return nil, err
	}
	err = store.Put("key1", "value1")
	if err != nil {
		return nil, err
	}
	err = store.Put("key2", "value2")
	if err != nil {
		return nil, err
	}
	err = store.Put("key3", "value3")
	if err != nil {
		return nil, err
	}
	err = store.Put("key4", "value4")
	if err != nil {
		return nil, err
	}
	err = store.Put("key5", "value5")
	if err != nil {
		return nil, err
	}
	return store, err
}

func TestStore_Open(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := createStoreWithData(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := store.Get("key1")
	if err != nil {
		t.Fatal(err)
	}
	if value != "value1" {
		t.Fatalf("expect value1, but got %s", value)
	}
	_, err = store.Get("keyNotFound")
	if !errors.Is(err, NotFound) {
		t.Fatalf("expect error, but got nil")
	}
	err = store.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestStore_PersistenceAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := createStoreWithData(path)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Close()
	if err != nil {
		t.Fatal(err)
	}

	secondStore, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := secondStore.Get("key1")
	if err != nil {
		t.Fatal(err)
	}
	if value != "value1" {
		t.Fatalf("expect value1, but got %s", value)
	}
	value, err = secondStore.Get("key5")
	if err != nil {
		t.Fatal(err)
	}
	if value != "value5" {
		t.Fatalf("expect value5, but got %s", value)
	}
	err = secondStore.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestStore_GetPut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := createStoreWithData(path)
	if err != nil {
		t.Fatal(err)
	}
	value1, err := store.Get("key1")
	if err != nil {
		t.Fatal(err)
	}
	if value1 != "value1" {
		t.Fatalf("expect value1, but got %s", value1)
	}
	err = store.Put("key1", "updatedValue")
	if err != nil {
		t.Fatal(err)
	}
	value2, err := store.Get("key1")
	if err != nil {
		t.Fatal(err)
	}
	if value2 != "updatedValue" {
		t.Fatalf("expect updatedValue, but got %s", value2)
	}
}

func TestStore_PutOverflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := createStoreWithData(path)
	if err != nil {
		t.Fatal(err)
	}
	sawOverflow := false
	for i := 0; i < 10000; i++ {
		if err = store.Put("key"+strconv.Itoa(i), "value"); err != nil {
			if !errors.Is(err, ErrorOverFlow) {
				t.Fatal(err)
			}
			sawOverflow = true
			break
		}
	}
	if !sawOverflow {
		t.Fatal("expected page to overflow but all the inserts succeeded")
	}
	err = store.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGet_Two_Level_Page(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	pager, err := newPager(path)
	if err != nil {
		t.Fatal(err)
	}
	ln1 := newLeafNode() // page id 0
	ln1PageID := pager.allocatePage()
	ln2 := newLeafNode() //page id 1
	ln2PageID := pager.allocatePage()
	in0 := newInternalNode() //page id 2
	in0PageID := pager.allocatePage()
	ln1.nextPageID = ln2PageID

	ok, err := ln1.insertLeaf("a", "a_value")
	if !ok {
		t.Fatal(err)
	}
	ok, err = ln1.insertLeaf("b", "b_value")
	if !ok {
		t.Fatal(err)
	}
	ok, err = ln1.insertLeaf("c", "c_value")
	if !ok {
		t.Fatal(err)
	}

	encodeLn1, err := ln1.encodeLeaf()
	if err != nil {
		t.Fatal(err)
	}

	err = pager.writePage(encodeLn1, ln1PageID)
	if err != nil {
		t.Fatal(err)
	}

	ok, err = ln2.insertLeaf("m", "m_value")
	if !ok {
		t.Fatal(err)
	}
	ok, err = ln2.insertLeaf("n", "n_value")
	if !ok {
		t.Fatal(err)
	}
	ok, err = ln2.insertLeaf("o", "o_value")
	if !ok {
		t.Fatal(err)
	}
	encodedLn2, err := ln2.encodeLeaf()
	if err != nil {
		t.Fatal(err)
	}
	err = pager.writePage(encodedLn2, ln2PageID)
	if err != nil {
		t.Fatal(err)
	}

	in0.children = append(in0.children, ln1PageID)
	in0.children = append(in0.children, ln2PageID)
	in0.keys = append(in0.keys, "m")

	encodedIn0, err := in0.encodeInternal()
	if err != nil {
		t.Fatal(err)
	}
	err = pager.writePage(encodedIn0, in0PageID)

	store := Store{
		pager:      pager,
		rootPageID: in0PageID,
	}

	val, err := store.Get("a")
	if err != nil {
		t.Fatal(err)
	}
	if val != "a_value" {
		t.Fatalf("expect a_value, but got %s", val)
	}

	val, err = store.Get("n")
	if err != nil {
		t.Fatal(err)
	}
	if val != "n_value" {
		t.Fatalf("expect n_value, but got %s", val)
	}

	val, err = store.Get("z")
	if !errors.Is(err, NotFound) {
		t.Fatalf("expect Not found, but got %v", err)
	}

	val, err = store.Get("aa")
	if !errors.Is(err, NotFound) {
		t.Fatalf("expect Not found, but got %v", err)
	}
}
