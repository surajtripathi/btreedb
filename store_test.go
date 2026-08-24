package btreestore

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
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

const baseKey = "keydkjngdfshfkhkhfjhkdfhksdhvkdhkdfkdhfsdhfkdhfkdsjhfksdfhksfsasjashkdfhxkcdkjngdfshfkhkhfjhkdfhksdhvkdhkdfkdhfsdhfkdhfkdsjhfksdfhksfsasjashkdfhxkc"
const baseValue = "i am a very very laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaage value, i am a very very laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaage value, i am a very very laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaage value"

type TestData struct {
	leafKeys             []string
	leafValues           []string
	leafDepths           []int
	internalKeySize      []int
	internalChildrenSize []int
}

func dumpTree(s *Store, pageID uint64, depth int, td *TestData) error {
	page, err := s.pager.readPage(pageID)
	if err != nil {
		return err
	}
	switch nodeType(page) {
	case LeafNodeType:
		leaf, err := decodeLeaf(page)
		if err != nil {
			return err
		}
		for i := 0; i < len(leaf.kv); i++ {
			td.leafKeys = append(td.leafKeys, leaf.kv[i].key)
			td.leafValues = append(td.leafValues, leaf.kv[i].value)
		}
		td.leafDepths = append(td.leafDepths, depth)
	case InternalNodeType:
		in, err := decodeInternal(page)
		if err != nil {
			return err
		}
		td.internalKeySize = append(td.internalKeySize, len(in.keys))
		td.internalChildrenSize = append(td.internalChildrenSize, len(in.children))
		for _, childID := range in.children {
			if err := dumpTree(s, childID, depth+1, td); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestStore_TreeStructureAfterManyInserts(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	size := 3001

	keys := make([]string, size)
	values := make([]string, size)

	for i := 0; i < size; i++ {
		key := baseKey + strconv.Itoa(i)
		value := baseValue + strconv.Itoa(i)
		if err = store.Put(key, value); err != nil {
			t.Fatal(err)
		}
		keys[i] = key
		values[i] = value
	}
	// validate key, value check using get
	for i := 0; i < size; i++ {
		key := baseKey + strconv.Itoa(i)
		expectedValue := baseValue + strconv.Itoa(i)
		gotValue, err := store.Get(key)
		if err != nil {
			t.Fatal(err)
		}
		if gotValue != expectedValue {
			t.Fatalf("expect %s, but got %s", expectedValue, gotValue)
		}
	}

	td := &TestData{
		leafKeys:             make([]string, 0),
		leafValues:           make([]string, 0),
		leafDepths:           make([]int, 0),
		internalKeySize:      make([]int, 0),
		internalChildrenSize: make([]int, 0),
	}
	err = dumpTree(store, store.rootPageID, 0, td)

	// assert: leaf entry count sums to number of keys inserted
	slices.Sort(td.leafKeys)
	slices.Sort(keys)
	if !reflect.DeepEqual(td.leafKeys, keys) {
		t.Fatalf("expect %v, but got %v", keys, td.leafKeys)
	}

	slices.Sort(td.leafValues)
	slices.Sort(values)
	if !reflect.DeepEqual(td.leafValues, values) {
		t.Fatalf("expect %v, but got %v", values, td.leafValues)
	}

	// assert: all leaf depths equal
	leafDepth := td.leafDepths[0]
	for i := 1; i < len(td.leafDepths); i++ {
		if leafDepth != td.leafDepths[i] {
			t.Fatalf("leaf nodes are expected to be at same lvel, expect %d, but got %d", leafDepth, td.leafDepths[i])
		}
	}

	// assert: every internal node satisfies len(children) == len(keys)+1
	for i := 0; i < len(td.internalChildrenSize); i++ {
		if td.internalChildrenSize[i] != td.internalKeySize[i]+1 {
			t.Fatalf("internal nodes keys size should be 1 less than internal nodes childrens")
		}
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
	in0, err := newInternalNode(ln1PageID, ln2PageID, "m") //page id 2
	if err != nil {
		t.Fatal(err)
	}
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
