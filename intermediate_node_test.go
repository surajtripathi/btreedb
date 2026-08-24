package btreestore

import (
	"errors"
	"log"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

func TestInternalNode_Encode_Decode(t *testing.T) {
	in, err := newInternalNode(1, 2, "keys")
	if err != nil {
		t.Fatal(err)
	}
	in.keys = append(in.keys, "key2")
	in.keys = append(in.keys, "key3")
	in.keys = append(in.keys, "key4")
	in.children = append(in.children, 3)
	in.children = append(in.children, 4)
	in.children = append(in.children, 5)

	encodeIntermediateNodeBuff, err := in.encodeInternal()
	if err != nil {
		t.Fatalf("encodeIntermediateNode failed: %v", err)
	}

	inDecoded, err := decodeInternal(encodeIntermediateNodeBuff)
	if err != nil {
		t.Fatalf("decodeIntermediateNode failed: %v", err)
	}

	if len(in.children) != len(inDecoded.children) {
		t.Fatalf("children length mismatch: %d != %d", len(in.children), len(inDecoded.children))
	}

	if len(in.keys) != len(inDecoded.keys) {
		t.Fatalf("keys length mismatch: %d != %d", len(in.keys), len(inDecoded.keys))
	}
	for i := 0; i < len(in.children); i++ {
		if inDecoded.children[i] != in.children[i] {
			t.Fatalf("children mismatch: %d != %d", inDecoded.children[i], in.children[i])
		}
	}
	for i := 0; i < len(in.keys); i++ {
		if inDecoded.keys[i] != in.keys[i] {
			t.Fatalf("keys mismatch: %s != %s", inDecoded.keys[i], in.keys[i])
		}
	}
}

func TestInternalNode_Encode_Decode_Zero_Size(t *testing.T) {
	in := &InternalNode{
		keys:     make([]string, 0),
		children: make([]uint64, 0),
	}
	in.children = append(in.children, 5)

	encodeIntermediateNodeBuff, err := in.encodeInternal()
	if err != nil {
		t.Fatalf("encodeIntermediateNode failed: %v", err)
	}

	inDecoded, err := decodeInternal(encodeIntermediateNodeBuff)
	if err != nil {
		t.Fatalf("decodeIntermediateNode failed: %v", err)
	}

	if len(in.children) != len(inDecoded.children) {
		t.Fatalf("children length mismatch: %d != %d", len(in.children), len(inDecoded.children))
	}

	if len(in.keys) != len(inDecoded.keys) {
		t.Fatalf("keys length mismatch: %d != %d", len(in.keys), len(inDecoded.keys))
	}
}

func TestInternalNode_Encode_Overflow(t *testing.T) {
	in, err := newInternalNode(0, 1, "keys0")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 1000; i++ {
		in.keys = append(in.keys, "key"+strconv.Itoa(i))
		in.children = append(in.children, uint64(i+1))
	}
	_, err = in.encodeInternal()
	if !errors.Is(err, ErrorOverFlow) {
		t.Fatalf("encodeIntermediateNode failed: %v", err)
	}
}

func TestInternalNode_Search(t *testing.T) {
	in, err := newInternalNode(0, 1, "key1")
	if err != nil {
		t.Fatal(err)
	}
	in.keys = append(in.keys, "key2")
	in.keys = append(in.keys, "key3")
	in.keys = append(in.keys, "key4")

	in.children = append(in.children, 2)
	in.children = append(in.children, 3)
	in.children = append(in.children, 4)

	index := in.searchInternal("key0")
	// index should be zero, index zero points to children[0]
	if index != 0 {
		t.Fatalf("searchInternal failed: %v", index)
	}
	index = in.searchInternal("key1")
	// index should be 1, index 1 points to children[1]
	if index != 1 {
		t.Fatalf("searchInternal failed: %v", index)
	}
	index = in.searchInternal("key12")
	// index should still 1, index 1 points to children[1]
	if index != 1 {
		t.Fatalf("searchInternal failed: %v", index)
	}
	index = in.searchInternal("key41")
	// index should still 4, index 4 points to children[4]
	if index != 4 {
		t.Fatalf("searchInternal failed: %v", index)
	}

	index = in.searchInternal("key4")
	// index should be 4, index 4 points to children[4]
	if index != 4 {
		t.Fatalf("searchInternal failed: %v", index)
	}
}

func TestInternalNode_Insert(t *testing.T) {
	in, err := newInternalNode(0, 1, "key1")
	if err != nil {
		t.Fatal(err)
	}
	//should be inserted at end
	err = in.insertInternal("key2", 2)
	if err != nil {
		t.Fatal(err)
	}

	keys := []string{"key1", "key2"}
	children := []uint64{0, 1, 2}

	if len(in.children) != 3 {
		t.Fatalf("children length mismatch: %d != %d", len(in.children), 3)
	}
	if len(in.keys) != 2 {
		t.Fatalf("keys length mismatch: %d != %d", len(in.keys), 2)
	}

	if !slices.Equal(in.keys, keys) {
		t.Fatalf("keys mismatch: %v != %v", in.keys, keys)
	}
	if !slices.Equal(in.children, children) {
		t.Fatalf("children mismatch: %v != %v", in.children, children)
	}

	// should be inserted at end again, just increasing the size without validation
	err = in.insertInternal("key3", 3)
	if err != nil {
		t.Fatal(err)
	}
	// should be inserted after the key2
	err = in.insertInternal("key21", 34)
	if err != nil {
		t.Fatal(err)
	}

	keys = []string{"key1", "key2", "key21", "key3"}
	children = []uint64{0, 1, 2, 34, 3}

	if !slices.Equal(in.keys, keys) {
		t.Fatalf("keys mismatch: %v != %v", in.keys, keys)
	}

	if !slices.Equal(in.children, children) {
		t.Fatalf("children mismatch: %v != %v", in.children, children)
	}

	//should  be inserted in the beginning
	err = in.insertInternal("key0", 10)
	if err != nil {
		t.Fatal(err)
	}

	keys = []string{"key0", "key1", "key2", "key21", "key3"}
	children = []uint64{0, 10, 1, 2, 34, 3}

	if !slices.Equal(in.keys, keys) {
		t.Fatalf("keys mismatch: %v != %v", in.keys, keys)
	}
	if !slices.Equal(in.children, children) {
		t.Fatalf("children mismatch: %v != %v", in.children, children)
	}
}

func getInternalNode(t *testing.T) *InternalNode {
	in, err := newInternalNode(0, 1, "key1")
	if err != nil {
		t.Fatal(err)
	}
	err = in.insertInternal("key2", 2)
	if err != nil {
		t.Fatal(err)
	}
	err = in.insertInternal("key3", 3)
	if err != nil {
		t.Fatal(err)
	}
	err = in.insertInternal("key4", 4)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestInternalNode_Split_AtStart(t *testing.T) {
	in := getInternalNode(t)
	// split from beginning

	key := "key0"
	pageID := uint64(10)
	// key0, key1, key2, key3, key4 ==> [key0, key1] [key3, key4] promotionKey = key2
	// 0, 1,    2,    3,    4,    5 ==> [0, 10,    1] [2,3,    4]

	lk := []string{"key0", "key1"}
	rk := []string{"key3", "key4"}

	lc := []uint64{0, 10, 1}
	rc := []uint64{2, 3, 4}
	pk := "key2"
	left, right, promotedKey, err := in.splitInternal(key, pageID)
	if err != nil {
		log.Fatal(err)
	}

	if !reflect.DeepEqual(left.keys, lk) {
		t.Fatalf("left.keys mismatch: %v != %v", left.keys, lk)
	}
	if !reflect.DeepEqual(left.children, lc) {
		t.Fatalf("left.children mismatch: %v != %v", left.children, lc)
	}
	if !reflect.DeepEqual(right.keys, rk) {
		t.Fatalf("right.keys mismatch: %v != %v", right.keys, rk)
	}
	if !reflect.DeepEqual(right.children, rc) {
		t.Fatalf("right.children mismatch: %v != %v", right.children, rc)
	}
	if pk != promotedKey {
		t.Fatalf("pk mismatch: %v != %v", pk, promotedKey)
	}

}

func TestInternalNode_Split_AtMiddle(t *testing.T) {
	in := getInternalNode(t)
	// split from beginning

	key := "key21"
	pageID := uint64(10)
	// key0, key1, key2, key3, key4 ==> [key0, key1] [key3, key4] promotionKey = key2
	// 0, 1,    2,    3,    4,    5 ==> [0, 10,    1] [2,3,    4]

	lk := []string{"key1", "key2"}
	rk := []string{"key3", "key4"}

	lc := []uint64{0, 1, 2}
	rc := []uint64{10, 3, 4}
	pk := "key21"
	left, right, promotedKey, err := in.splitInternal(key, pageID)
	if err != nil {
		log.Fatal(err)
	}

	if !reflect.DeepEqual(left.keys, lk) {
		t.Fatalf("left.keys mismatch: %v != %v", left.keys, lk)
	}
	if !reflect.DeepEqual(left.children, lc) {
		t.Fatalf("left.children mismatch: %v != %v", left.children, lc)
	}
	if !reflect.DeepEqual(right.keys, rk) {
		t.Fatalf("right.keys mismatch: %v != %v", right.keys, rk)
	}
	if !reflect.DeepEqual(right.children, rc) {
		t.Fatalf("right.children mismatch: %v != %v", right.children, rc)
	}
	if pk != promotedKey {
		t.Fatalf("pk mismatch: %v != %v", pk, promotedKey)
	}

}

func TestInternalNode_Split_AtEnd(t *testing.T) {
	in := getInternalNode(t)
	// split from beginning

	key := "key41"
	pageID := uint64(10)
	// key0, key1, key2, key3, key4 ==> [key0, key1] [key3, key4] promotionKey = key2
	// 0, 1,    2,    3,    4,    5 ==> [0, 10,    1] [2,3,    4]

	lk := []string{"key1", "key2"}
	rk := []string{"key4", "key41"}

	lc := []uint64{0, 1, 2}
	rc := []uint64{3, 4, 10}
	pk := "key3"
	left, right, promotedKey, err := in.splitInternal(key, pageID)
	if err != nil {
		log.Fatal(err)
	}

	if !reflect.DeepEqual(left.keys, lk) {
		t.Fatalf("left.keys mismatch: %v != %v", left.keys, lk)
	}
	if !reflect.DeepEqual(left.children, lc) {
		t.Fatalf("left.children mismatch: %v != %v", left.children, lc)
	}
	if !reflect.DeepEqual(right.keys, rk) {
		t.Fatalf("right.keys mismatch: %v != %v", right.keys, rk)
	}
	if !reflect.DeepEqual(right.children, rc) {
		t.Fatalf("right.children mismatch: %v != %v", right.children, rc)
	}
	if pk != promotedKey {
		t.Fatalf("pk mismatch: %v != %v", pk, promotedKey)
	}

}
