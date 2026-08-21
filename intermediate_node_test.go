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
