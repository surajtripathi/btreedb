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
