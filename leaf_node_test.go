package btreestore

import (
	"errors"
	"strconv"
	"testing"
)

var largeValue = `The History and Evolution of the Internet
The internet is a vast global network. It links smaller computer networks together. Billions of people use it every day to talk, work, learn, and play. The story of how this system grew is long and very interesting. It started as a small project in the United States and grew into a worldwide web.
The Early Days and ARPANET
In the late 1960s, the United States Department of Defense wanted a safe way for computers to share data. The Advanced Research Projects Agency, known as ARPA, created ARPANET. This project used a new method called packet switching. Packet switching breaks data into small pieces. Each piece finds its own path through the network. Then, the pieces join back together at the destination.
On October 29, 1969, the very first message moved from a computer at UCLA to a computer at Stanford University. The team tried to type the word "LOGIN". The system crashed after the first two letters. Still, the message "LO" was a massive success. It proved that computers could talk to each other over long distances.
Growth and the TCP/IP Protocol
During the 1970s, more computer networks appeared. However, these different networks could not talk to each other easily. Scientists Vint Cerf and Bob Kahn solved this problem. They created the Transmission Control Protocol and the Internet Protocol, known together as TCP/IP.
This new set of rules acted like a common language. Any computer on any network could use TCP/IP to understand other computers. On January 1, 1983, ARPANET officially switched to TCP/IP. This date is often seen as the true birthday of the modern internet. Around this time, people started to use the word "internet" to describe this connected group of networks.
The World Wide Web
For many years, the internet was mostly text. Only scientists, university students, and the military used it. That changed in 1989. A British scientist named Tim Berners-Lee worked at CERN in Switzerland. He invented the World Wide Web.
He created three main tools:

• HTML: A simple language to write web pages.
• URI/URL: A way to find where a page lives.
• HTTP: A rule for moving pages from a server to a user.

In 1991, the World Wide Web became open to the public. People could now use web browsers to see pictures, click links, and read pages easily. Websites grew very fast during the 1990s. Companies started to sell goods online. This time was known as the dot-com boom.
Web 2.0 and Social Media
In the 2000s, the internet changed again. The first years of the web let people only read pages. The new era, often called Web 2.0, let people create content too. Users could upload videos, write blogs, and share ideas.
Social media sites appeared. People could talk to friends across the planet instantly. Smartphones came out in the late 2000s. People started to carry the internet in their pockets. Wi-Fi and mobile data made the internet available almost anywhere, all the time.
Modern Cloud and the Future
Today, the internet is more than just web pages. It is a massive cloud of data. Huge server farms store movies, songs, and files for millions of users. Smart devices, cars, and home appliances now connect to the network. This is called the Internet of Things.
New tools like artificial intelligence use the internet to learn and answer questions. The internet changed how humans live, work, and think. It turned a divided world into a single connected community.

AI responses may include mistakes.`

// — multiple entries, checks full equality (length + every field, in order).
func TestEncodeDecodeRoundTrip(t *testing.T) {
	node := newLeafNode()

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
	node := newLeafNode()
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
	node := newLeafNode()

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
	node := newLeafNode()

	for i := 0; i < pageSize; i++ {
		node.kv = append(node.kv, keyValue{key: strconv.Itoa(i), value: strconv.Itoa(i)})
	}

	_, err := node.encode()
	if err == nil || err.Error() != "overflow error" {
		t.Fatalf("Should have thrown overflow error")
	}
}

func TestLeafNodeSearch(t *testing.T) {
	node := newLeafNode()

	node.kv = append(node.kv, keyValue{key: "hello1", value: "world1"})
	node.kv = append(node.kv, keyValue{key: "hello2", value: "world2"})
	node.kv = append(node.kv, keyValue{key: "hello3", value: "world3"})

	index, ok := node.search("hello1")
	if !ok {
		t.Fatalf("key not found %d", index)
	}
	if index != 0 {
		t.Fatalf("index should be 0, got %d", index)
	}

	if node.kv[index].key != "hello1" {
		t.Fatalf("key should be 'hello1', got %s", node.kv[index].key)
	}

	index, ok = node.search("hello2")
	if !ok {
		t.Fatal("key not found")
	}
	if index != 1 {
		t.Fatalf("index should be 1, got %d", index)
	}

	if node.kv[index].key != "hello2" {
		t.Fatalf("key should be 'hello1', got %s", node.kv[index].key)
	}

	index, ok = node.search("hello3")
	if !ok {
		t.Fatal("key not found")
	}
	if index != 2 {
		t.Fatalf("index should be 2, got %d", index)
	}

	if node.kv[index].key != "hello3" {
		t.Fatalf("key should be 'hello1', got %s", node.kv[index].key)
	}

	index, ok = node.search("hello11")
	if ok {
		t.Fatal("key not found")
	}
	if index != 1 {
		t.Fatalf("index should be 1, got %d", index)
	}
	if node.kv[index].key == "hello11" {
		t.Fatalf("search returned wrong key, expected \"hello11\"")
	}

}

func TestLeafNodeInsertIntoEmptyNode(t *testing.T) {
	node := newLeafNode()

	ok, err := node.insert("hello11", "value11")
	if err != nil {
		t.Fatal(err)
	}
	if ok != true {
		t.Fatalf("ok should be true, got %t", ok)
	}
	if len(node.kv) != 1 {
		t.Fatalf("len(node.kv) should be 1, got %d", len(node.kv))
	}
	if node.kv[0].key != "hello11" {
		t.Fatalf("node.kv[0].key should be 'hello11', got %s", node.kv[0].key)
	}
}
func TestLeafNodeInsertIntoFrontMiddleAndEnd(t *testing.T) {
	node := newLeafNode()

	// inset empty
	ok, err := node.insert("hello1", "value1")
	ok, err = node.insert("hello2", "value2")
	ok, err = node.insert("hello3", "value3")
	ok, err = node.insert("hello4", "value4")
	ok, err = node.insert("hello5", "value5")
	if err != nil {
		t.Fatal(err)
	}

	// insert front
	ok, err = node.insert("hello0", "value0")
	if err != nil {
		t.Fatal(err)
	}
	if ok != true {
		t.Fatalf("ok should be true, got %t", ok)
	}
	if node.kv[0].key != "hello0" {
		t.Fatalf("node.kv[0].key should be 'hello11', got %s", node.kv[0].key)
	}
	// insert middle
	ok, err = node.insert("hello21", "value21")
	if err != nil {
		t.Fatal(err)
	}
	if ok != true {
		t.Fatalf("ok should be true, got %t", ok)
	}
	// 0, 1, 2, 21, 3, 4, 5
	if node.kv[3].key != "hello21" {
		t.Fatalf("node.kv[3].key should be 'hello11', got %s", node.kv[3].key)
	}

	// insert end
	ok, err = node.insert("hello51", "value51")
	if err != nil {
		t.Fatal(err)
	}
	if ok != true {
		t.Fatalf("ok should be true, got %t", ok)
	}
	// 0, 1, 2, 21, 3, 4, 5, 51
	if node.kv[7].key != "hello51" {
		t.Fatalf("node.kv[7].key should be 'hello11', got %s", node.kv[7].key)
	}
}
func TestLeafNodeInsertUpdateExistingKeyWithLargerValue(t *testing.T) {
	node := newLeafNode()

	// inset empty
	ok, err := node.insert("hello1", "value1")
	ok, err = node.insert("hello2", "value2")
	ok, err = node.insert("hello3", "value3")
	ok, err = node.insert("hello4", "value4")
	ok, err = node.insert("hello5", "value5")
	if err != nil {
		t.Fatal(err)
	}

	// override
	ok, err = node.insert("hello2", "updatedLargerValue")
	if err != nil {
		t.Fatal(err)
	}
	if ok != true {
		t.Fatalf("ok should be true, got %t", ok)
	}
	if node.kv[1].key != "hello2" || node.kv[1].value != "updatedLargerValue" {
		t.Fatalf("key, value should be 'hello2', updatedLargerValue, but got %s, %s", node.kv[1].key, node.kv[1].value)
	}
}
func TestLeafNodeInsertUpdateExistingKeWithSmallerValue(t *testing.T) {
	node := newLeafNode()

	// inset empty
	ok, err := node.insert("hello1", "value1")
	ok, err = node.insert("hello2", "I am a very very large value")
	ok, err = node.insert("hello3", "value3")
	ok, err = node.insert("hello4", "value4")
	ok, err = node.insert("hello5", "value5")
	if err != nil {
		t.Fatal(err)
	}

	// override
	ok, err = node.insert("hello2", "small value")
	if err != nil {
		t.Fatal(err)
	}
	if ok != true {
		t.Fatalf("ok should be true, got %t", ok)
	}
	if node.kv[1].key != "hello2" || node.kv[1].value != "small value" {
		t.Fatalf("key, value should be 'hello2', small value, but got %s, %s", node.kv[1].key, node.kv[1].value)
	}
}

func TestLeafNodeInsertOverflow(t *testing.T) {
	node := newLeafNode()

	// inset empty
	//fmt.Printf("slotOffset %d, freePageOffset %d\n", node.slotOffset, node.freePageOffset)

	ok, err := node.insert("hello1", "value1")
	if err != nil {
		t.Fatal(err)
	}

	// override
	//fmt.Printf("slotOffset %d, freePageOffset %d\n", node.slotOffset, node.freePageOffset)

	//fmt.Printf("string length %d\n", len(largeValue))
	ok, err = node.insert("hello2", largeValue)
	//fmt.Printf("slotOffset %d, freePageOffset %d\n", node.slotOffset, node.freePageOffset)
	ok, err = node.insert("hello3", largeValue)
	if err == nil || ok {
		t.Fatalf("insert should fail on overflow")
	}
	if !errors.Is(err, ErrorOverFlow) {
		t.Fatalf("insert should fail on overflow")
	}
}

func TestLeafNodeInsertOverrideOverflow(t *testing.T) {
	node := newLeafNode()

	// inset empty
	//fmt.Printf("slotOffset %d, freePageOffset %d\n", node.slotOffset, node.freePageOffset)

	ok, err := node.insert("hello1", "value1")
	if err != nil {
		t.Fatal(err)
	}

	// override
	//fmt.Printf("slotOffset %d, freePageOffset %d\n", node.slotOffset, node.freePageOffset)

	//fmt.Printf("string length %d\n", len(largeValue))
	ok, err = node.insert("hello2", largeValue)
	//fmt.Printf("slotOffset %d, freePageOffset %d\n", node.slotOffset, node.freePageOffset)
	ok, err = node.insert("hello3", "small Value")
	if err != nil {
		t.Fatal(err)
	}
	ok, err = node.insert("hello3", largeValue)
	if err == nil || ok {
		t.Fatalf("insert should fail on overflow")
	}
	if !errors.Is(err, ErrorOverFlow) {
		t.Fatalf("insert should fail on overflow")
	}
}

func TestLeafNodeInsert(t *testing.T) {
	node := newLeafNode()

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

	oldLen := len(decodedNode.kv)
	ok, err := decodedNode.insert("hello11", "value11")
	if err != nil {
		t.Fatal(err)
	}
	//for i := 0; i < len(decodedNode.kv); i++ {
	//	fmt.Printf("%s: %s\n", decodedNode.kv[i].key, decodedNode.kv[i].value)
	//}
	if ok != true {
		t.Fatalf("ok should be true, got %t", ok)
	}
	if len(decodedNode.kv) != oldLen+1 {
		t.Fatalf("len(node.kv) should be %d, got %d", oldLen+1, len(decodedNode.kv))
	}
	if decodedNode.kv[1].key != "hello11" {
		t.Fatalf("node.kv[1].key should be 'hello11', got %s", decodedNode.kv[1].key)
	}

}

func TestLeafNodeInsertOffsetBookkeeping(t *testing.T) {
	node := newLeafNode()

	// pick keys/values, compute expected cellSize by hand for each
	key1 := "hello1"
	value1 := "value1"
	_, err := node.insert(key1, value1)
	if err != nil {
		t.Fatal(err)
	}
	freePageOffset := pageEnd - (kvLengthStoreSize + len(key1) + kvLengthStoreSize + len(value1))
	slotOffset := slotDirStart + 2
	// assert exact offsets here, using numbers you computed by hand above
	if node.freePageOffset != uint16(freePageOffset) /* your computed value */ {
		t.Fatalf("freePageOffset = %d, expected %d", node.freePageOffset, freePageOffset /* expected */)
	}
	if node.slotOffset != uint16(slotOffset) /* your computed value */ {
		t.Fatalf("slotOffset = %d, expected %d", node.slotOffset, slotOffset /* expected */)
	}

	// second insert, recompute expected offsets from the first insert's result
	key2 := "hello2"
	value2 := "value2"
	_, err = node.insert(key2, value2)
	if err != nil {
		t.Fatal(err)
	}
	freePageOffset -= kvLengthStoreSize + len(key2) + kvLengthStoreSize + len(value2)
	slotOffset += 2
	if node.freePageOffset != uint16(freePageOffset) {
		t.Fatalf("freePageOffset = %d, expected %d", node.freePageOffset, freePageOffset)
	}
	if node.slotOffset != uint16(slotOffset) {
		t.Fatalf("slotOffset = %d, expected %d", node.slotOffset, slotOffset)
	}

	// now do one UPDATE (existing key, different-length value) and verify
	// freePageOffset moves by the delta (new-old), slotOffset stays unchanged
	updatedValue2 := "updatedValue2"
	_, err = node.insert(key2, updatedValue2)
	if err != nil {
		t.Fatal(err)
	}
	freePageOffset -= len(updatedValue2) - len(value2)
	if node.freePageOffset != uint16(freePageOffset) {
		t.Fatalf("freePageOffset = %d, expected %d", node.freePageOffset, freePageOffset)
	}
	if node.slotOffset != uint16(slotOffset) {
		t.Fatalf("slotOffset = %d, expected %d", node.slotOffset, slotOffset)
	}
}
