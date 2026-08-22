package btreestore

import (
	"errors"
	"strconv"
	"testing"
)

func TestInternalNode_Encode_Decode(t *testing.T) {
	in := newInternalNode()
	in.keys = append(in.keys, "key1")
	in.keys = append(in.keys, "key2")
	in.keys = append(in.keys, "key3")
	in.keys = append(in.keys, "key4")
	in.children = append(in.children, 1)
	in.children = append(in.children, 2)
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
	in := newInternalNode()
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
	in := newInternalNode()
	for i := 0; i < 1000; i++ {
		in.keys = append(in.keys, "key"+strconv.Itoa(i))
		in.children = append(in.children, uint64(i))
	}
	in.children = append(in.children, uint64(1000))
	_, err := in.encodeInternal()
	if !errors.Is(err, ErrorOverFlow) {
		t.Fatalf("encodeIntermediateNode failed: %v", err)
	}
}

func TestInternalNode_Search(t *testing.T) {
	in := newInternalNode()
	in.keys = append(in.keys, "key1")
	in.keys = append(in.keys, "key2")
	in.keys = append(in.keys, "key3")
	in.keys = append(in.keys, "key4")

	in.children = append(in.children, 0)
	in.children = append(in.children, 1)
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
