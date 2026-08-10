package btreestore

import (
	"strconv"
	"testing"
)

// — multiple entries, checks full equality (length + every field, in order).
func TestEncodeDecodeRoundTrip(t *testing.T) {
	node := LeafNode{
		kv:         make([]keyValue, 0),
		nextPageID: 5,
	}

	node.kv = append(node.kv, keyValue{key: "hello1", value: "world1"})
	node.kv = append(node.kv, keyValue{key: "hello2", value: "world2"})
	node.kv = append(node.kv, keyValue{key: "hello3", value: "world3"})

	encodedPage, err := node.encode()

	if err != nil {
		t.Fatal(err)
	}

	decodedNode, err := decode(encodedPage)
	if err != nil {
		t.Fatal(err)
	}

	if len(decodedNode.kv) != len(node.kv) {
		t.Fatalf("decoded node has %d entries, expected %d", len(decodedNode.kv), len(node.kv))
	}

	for i := 0; i < len(node.kv); i++ {
		if node.kv[i].key != decodedNode.kv[i].key {
			t.Fatalf("decoded node has key %s, expected %s", node.kv[i].key, decodedNode.kv[i].key)
		}
		if node.kv[i].value != decodedNode.kv[i].value {
			t.Fatalf("decoded node has value %s, expected %s", node.kv[i].value, decodedNode.kv[i].value)
		}
	}
	if decodedNode.nextPageID != node.nextPageID {
		t.Fatalf("decoded node has next page id, expected %d", decodedNode.nextPageID)
	}
}

// — zero entries.
func TestEncodeDecodeEmptyNode(t *testing.T) {
	node := LeafNode{
		kv:         make([]keyValue, 0),
		nextPageID: 5,
	}
	encodedPage, err := node.encode()
	if err != nil {
		t.Fatal(err)
	}
	decodedNode, err := decode(encodedPage)
	if err != nil {
		t.Fatal(err)
	}
	if len(decodedNode.kv) != len(node.kv) {
		t.Fatalf("decoded node has %d entries, expected %d", len(decodedNode.kv), len(node.kv))
	}

	if len(decodedNode.kv) != 0 {
		t.Fatalf("decoded node has %d entries, expected 0", len(decodedNode.kv))
	}

	if decodedNode.nextPageID != node.nextPageID {
		t.Fatalf("decoded node has next page id, expected %d", decodedNode.nextPageID)
	}
}

// — empty string edge case.
func TestEncodeDecodeEmptyValue(t *testing.T) {
	node := LeafNode{
		kv:         make([]keyValue, 0),
		nextPageID: 5,
	}

	node.kv = append(node.kv, keyValue{key: "hello1"})
	encodedPage, err := node.encode()
	if err != nil {
		t.Fatal(err)
	}
	decodedNode, err := decode(encodedPage)
	if err != nil {
		t.Fatal(err)
	}
	if len(decodedNode.kv) != len(node.kv) {
		t.Fatalf("decoded node has %d entries, expected %d", len(decodedNode.kv), len(node.kv))
	}

	for i := 0; i < len(node.kv); i++ {
		if node.kv[i].key != decodedNode.kv[i].key {
			t.Fatalf("decoded node has key %s, expected %s", node.kv[i].key, decodedNode.kv[i].key)
		}
		if decodedNode.kv[i].value != "" {
			t.Fatalf("decoded node has value %s, expected \"\"", decodedNode.kv[i].value)
		}
	}
	if decodedNode.nextPageID != node.nextPageID {
		t.Fatalf("decoded node has next page id, expected %d", decodedNode.nextPageID)
	}
}

// — deliberately too much data, assert error returned.
func TestEncodeOverflow(t *testing.T) {
	node := LeafNode{
		kv:         make([]keyValue, pageSize),
		nextPageID: 5,
	}

	for i := 0; i < pageSize; i++ {
		node.kv[i] = keyValue{key: strconv.Itoa(i), value: strconv.Itoa(i)}
	}

	_, err := node.encode()
	if err == nil || err.Error() != "overflow error" {
		t.Fatalf("Should have thrown overflow error")
	}

}
