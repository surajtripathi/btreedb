package btreestore

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

func createStoreWithData(path string, walPath string) (*Store, error) {
	store, err := Open(StoreOptions{
		DBPath:                 path,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
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
	walPath := filepath.Join(dir, "test.wal")
	store, err := createStoreWithData(path, walPath)
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
	walPath := filepath.Join(dir, "test.wal")
	store, err := createStoreWithData(path, walPath)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Close()
	if err != nil {
		t.Fatal(err)
	}

	secondStore, err := Open(StoreOptions{
		DBPath:                 path,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
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
	walPath := filepath.Join(dir, "test.wal")
	store, err := createStoreWithData(path, walPath)
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
	btreePath := filepath.Join(dir, "test.db")
	walPath := filepath.Join(dir, "test.wal")
	store, err := Open(StoreOptions{
		DBPath:                 btreePath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
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
	err = store.Close()
	if err != nil {
		t.Fatal(err)
	}

	// open store again
	store, err = Open(StoreOptions{
		DBPath:                 btreePath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
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
	err = dumpTree(store, store.superBlock.rootPageID, 0, td)

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
		superBlock: newSuperBlockNode(in0PageID),
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

func TestWalRecover(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	walPath := filepath.Join(dir, "test.wal")

	store, err := Open(StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	keys := make([]string, 0)
	vals := make([]string, 0)

	for i := 0; i < 100; i++ {
		key := "a" + strconv.Itoa(i)
		value := "a_value" + strconv.Itoa(i)
		err = store.Put(key, value)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
		vals = append(vals, value)
	}

	crashKey := "crash-key"
	crashValue := "crash-value"

	//simulating crash, put is successful
	err = store.applyPut(crashKey, crashValue)
	if err != nil {
		t.Fatal(err)
	}
	keys = append(keys, crashKey)
	vals = append(vals, crashValue)

	td := &TestData{}
	err = dumpTree(store, store.superBlock.rootPageID, 0, td)
	if err != nil {
		t.Fatal(err)
	}

	slices.Sort(keys)
	slices.Sort(vals)

	if !reflect.DeepEqual(keys, td.leafKeys) {
		t.Fatalf("expect %v, but got %v", keys, td.leafKeys)
	}
	if !reflect.DeepEqual(vals, td.leafValues) {
		t.Fatalf("expect %v, but got %v", vals, td.leafValues)
	}

	// process crashes before wal truncate call, it will be re-applied when store opens again
	err = store.wal.Append(WalRecord{op: opPut, key: crashKey, value: crashValue})
	if err != nil {
		t.Fatal(err)
	}

	err = store.Close()
	if err != nil {
		t.Fatal(err)
	}

	// store.Open should re-apply the wal log
	store, err = Open(StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	td = &TestData{}
	err = dumpTree(store, store.superBlock.rootPageID, 0, td)
	if err != nil {
		t.Fatal(err)
	}

	// it should not have duplicate key and values as they were successfully inserted
	// so WalReply should be no op
	if !reflect.DeepEqual(keys, td.leafKeys) {
		t.Fatalf("expect %v, but got %v", keys, td.leafKeys)
	}
	if !reflect.DeepEqual(vals, td.leafValues) {
		t.Fatalf("expect %v, but got %v", vals, td.leafValues)
	}

	// simulate failed entry in the tree after getting written in wal
	crashKey2 := "crash-key2"
	crashVal2 := "crash-val2"

	// dont apply put, just apply wal.append
	err = store.wal.Append(WalRecord{op: opPut, key: crashKey2, value: crashVal2})
	if err != nil {
		t.Fatal(err)
	}

	val, err := store.Get(crashKey2)
	if err == nil || !errors.Is(err, NotFound) {
		t.Fatalf("expected NotFound before replay, but got value %v and error %v", val, err)
	}

	// not calling store.close as it will run the check pointing truncate the wal record so crashKey2 wont be found
	// in case of real crash checkpoint will not run so wal wont be truncated and crashKey2 will survive and reapplyied
	// during store opening
	err = store.pager.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = store.wal.Close()
	if err != nil {
		t.Fatal(err)
	}

	//opening store should recover from wal
	store, err = Open(StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	val, err = store.Get(crashKey2)
	if err != nil {
		t.Fatal(err)
	}
	if val != crashVal2 {
		t.Fatalf("expect %v, but got %v", crashVal2, val)
	}
}

func TestWalMultiKeyRecovery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	walPath := filepath.Join(dir, "test.wal")

	store, err := Open(StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	keys := make([]string, 0)
	vals := make([]string, 0)

	for i := 0; i < 15; i++ {
		key := "a" + strconv.Itoa(i)
		value := "a_value" + strconv.Itoa(i)
		err = store.Put(key, value)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
		vals = append(vals, value)
	}

	// not calling store.close as it will run the check pointing truncate the wal record so crashKey2 wont be found
	// in case of real crash checkpoint will not run so wal wont be truncated and crashKey2 will survive and reapplyied
	// during store opening
	err = store.pager.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = store.wal.Close()
	if err != nil {
		t.Fatal(err)
	}

	// store.Open should re-apply the wal log, all 15
	store, err = Open(StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	td := &TestData{}
	err = dumpTree(store, store.superBlock.rootPageID, 0, td)
	if err != nil {
		t.Fatal(err)
	}

	slices.Sort(keys)
	slices.Sort(vals)

	// it should not have duplicate key and values as they were successfully inserted
	// so WalReply should be no op
	if !reflect.DeepEqual(keys, td.leafKeys) {
		t.Fatalf("expect %v, but got %v", keys, td.leafKeys)
	}
	if !reflect.DeepEqual(vals, td.leafValues) {
		t.Fatalf("expect %v, but got %v", vals, td.leafValues)
	}

	// test wal ordering post crash recovery

	err = store.Put("a", "a_value 1")
	if err != nil {
		t.Fatal(err)
	}
	err = store.Put("a", "a_value 2")
	if err != nil {
		t.Fatal(err)
	}
	err = store.Put("a", "a_value 3")
	if err != nil {
		t.Fatal(err)
	}
	// db should only have the last value
	keys = append(keys, "a")
	vals = append(vals, "a_value 3")

	// not calling store.close as it will run the check pointing truncate the wal record so crashKey2 wont be found
	// in case of real crash checkpoint will not run so wal wont be truncated and crashKey2 will survive and reapplyied
	// during store opening
	err = store.pager.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = store.wal.Close()
	if err != nil {
		t.Fatal(err)
	}

	store, err = Open(StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	td = &TestData{}
	err = dumpTree(store, store.superBlock.rootPageID, 0, td)
	if err != nil {
		t.Fatal(err)
	}

	slices.Sort(keys)
	slices.Sort(vals)

	// it should not have duplicate key and values as they were successfully inserted
	// so WalReply should be no op
	if !reflect.DeepEqual(keys, td.leafKeys) {
		t.Fatalf("expect %v, but got %v", keys, td.leafKeys)
	}
	if !reflect.DeepEqual(vals, td.leafValues) {
		t.Fatalf("expect %v, but got %v", vals, td.leafValues)
	}
}

func TestRangeGet(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	walPath := filepath.Join(dir, "test.wal")

	store, err := Open(StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	// do range scan on empty db, should return zero result

	kv, err := store.RangeGet("a", "z")
	if err != nil {
		t.Fatal(err)
	}
	if len(kv) != 0 {
		t.Fatalf("expect empty, but got %v", kv)
	}

	// do a range query on not empty db
	keyValues := make([]keyValue, 0)

	for i := 1000; i < 10000; i++ {
		// large value, that gurantees the next page movement
		key := baseKey + strconv.Itoa(i)
		value := baseValue + strconv.Itoa(i)
		err = store.Put(key, value)
		if err != nil {
			t.Fatal(err)
		}
		keyValues = append(keyValues, keyValue{key: key, value: value})
	}

	//slices.SortFunc(keyValues, func(a, b keyValue) int {
	//	return cmp.Compare(a.value, b.value)
	//})

	// data exists in between, start matches with left most key, end matches with right most key
	startIndex, endIndex := 9, 50
	kv, err = store.RangeGet(keyValues[startIndex].key, keyValues[endIndex].key)
	if err != nil {
		t.Fatal(err)
	}
	if len(kv) != len(keyValues[startIndex:endIndex+1]) {
		t.Fatalf("expect %v, but got %v", keyValues[startIndex:endIndex+1], kv)
	}
	if !reflect.DeepEqual(kv, keyValues[startIndex:endIndex+1]) {
		t.Fatalf("expect %v, but got %v", keyValues[startIndex:endIndex+1], kv)
	}

	// data exists in between, start does not match with left most key, end does not match with right most key
	startIndex, endIndex = 50, 90

	startKey := keyValues[startIndex].key + "0" // a10500
	endKey := keyValues[endIndex].key + "0"     // a10900
	kv, err = store.RangeGet(startKey, endKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(kv) != len(keyValues[startIndex+1:endIndex+1]) {
		t.Fatalf("expect %v, but got %v", keyValues[startIndex+1:endIndex+1], kv)
	}
	if !reflect.DeepEqual(kv, keyValues[startIndex+1:endIndex+1]) {
		t.Fatalf("expect %v, but got %v", keyValues[startIndex+1:endIndex+1], kv)
	}

	// data does not exist in between of the tree, right key reaches beyond the db end,

	startKey = "aaaaaa"
	endKey = "zzzzzzz"

	kv, err = store.RangeGet(startKey, endKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(kv) != len(keyValues) {
		t.Fatalf("expect %v, but got %v", keyValues, kv)
	}
	if !reflect.DeepEqual(kv, keyValues) {
		t.Fatalf("expect %v, but got %v", keyValues, kv)
	}
}

func TestDeleteIncludingWalRecover(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	walPath := filepath.Join(dir, "test.wal")

	options := StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: 20,
	}
	store, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}

	keys := make([]string, 0)
	vals := make([]string, 0)

	for i := 0; i < 100; i++ {
		key := "a" + strconv.Itoa(i)
		value := "a_value" + strconv.Itoa(i)
		err = store.Put(key, value)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
		vals = append(vals, value)
	}
	//basic deleted test
	key := "a10"
	expectedValue := "a_value10"
	gotValue, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if expectedValue != gotValue {
		t.Fatalf("expect %v, but got %v", expectedValue, gotValue)
	}
	err = store.Delete(key)

	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(key)

	if err == nil || !errors.Is(err, NotFound) {
		t.Fatalf("expect %v, but got %v", NotFound, err)
	}

	crashDeleteKey := "crash-delete-key"
	crashDeleteValue := "crash-delete-value"

	err = store.Put(crashDeleteKey, crashDeleteValue)
	if err != nil {
		t.Fatalf("expect no error, but got %v", err)
	}
	//delete only wall , simulate crash
	err = store.wal.Append(WalRecord{
		op:  opDelete,
		key: crashDeleteKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	// only deleted in wal so get should return the value
	get, err := store.Get(crashDeleteKey)
	if err != nil {
		t.Fatal(err)
	}

	if get != crashDeleteValue {
		t.Fatalf("expect %v, but got %v", expectedValue, get)
	}
	err = store.wal.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = store.pager.Close()
	if err != nil {
		t.Fatal(err)
	}

	// opening store should recover the delete op

	store, err = Open(options)
	if err != nil {
		t.Fatal(err)
	}

	// should be found as wal replay should recover the delete
	_, err = store.Get(crashDeleteKey)
	if err == nil || !errors.Is(err, NotFound) {
		t.Fatalf("expect %v, but got %v", NotFound, err)
	}
}
