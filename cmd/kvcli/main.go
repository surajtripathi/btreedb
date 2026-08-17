package main

// A tiny REPL for poking at the store by hand.
//
// Try this to feel the WAL working:
//   1. go run ./cmd/kvcli data.wal
//   2. put foo bar
//   3. put baz qux
//   4. Ctrl+C to kill the process (simulating a crash)
//   5. go run ./cmd/kvcli data.wal
//   6. get foo        -> should still print "bar"
//
// Then try killing it mid-write by adding an artificial delay in
// WAL.Append (between the Write and the Sync) and killing at just the
// right moment - that's how you can reproduce a torn write and watch
// Replay() discard it safely.

import (
	"btreedb"
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
)

const dir = "store"
const filePrefix = "btree"
const walPath = "data.wal"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: kvcli <wal-file-path>")
		os.Exit(1)
	}

	store, err := btreestore.Open(path.Join(dir, filePrefix+".btree"))
	if err != nil {
		fmt.Println("failed to open store:", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Println("kvstore ready. commands: put <k> <v> | get <k> | del <k> | exit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "put":
			if len(fields) != 3 {
				fmt.Println("usage: put <key> <value>")
				continue
			}
			if err := store.Put(fields[1], fields[2]); err != nil {
				fmt.Println("error:", err)
			} else {
				fmt.Println("ok")
			}

		case "get":
			if len(fields) != 2 {
				fmt.Println("usage: get <key>")
				continue
			}
			val, err := store.Get(fields[1])
			if err != nil {
				fmt.Println("error:", err)
			} else {
				fmt.Println(val)
			}

		case "del":
			if len(fields) != 2 {
				fmt.Println("usage: del <key>")
				continue
			}
			if err := store.Delete(fields[1]); err != nil {
				fmt.Println("error:", err)
			} else {
				fmt.Println("ok")
			}

		case "exit", "quit":
			return

		default:
			fmt.Println("unknown command:", fields[0])
		}
	}
}
