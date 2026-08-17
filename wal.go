package btreestore

// wal.go implements a simple write-ahead log.
//
// Every mutation (Put or Delete) is appended to this log BEFORE it is
// applied to the in-memory map. If the process crashes, we can recover
// the exact state of the map by replaying the log from the start.
//
// Record format (all integers big-endian):
//
//   +-------------+--------+-----------+-----+-------------+-------+----------+
//   | totalLen(4) | op(1)  | keyLen(4) | key | valLen(4)   | value | crc32(4) |
//   +-------------+--------+-----------+-----+-------------+-------+----------+
//
// totalLen covers everything except itself and the checksum, so the
// reader knows how many bytes to read before validating.
//
// The checksum lets recovery detect a "torn write" - a record that was
// only partially flushed to disk when the process crashed. This is the
// same class of problem every real WAL (Postgres, RocksDB, etc.) has to
// handle: a crash mid-write must never corrupt state, it should just
// look like "that last write never happened".

import (
	"bufio"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
)

type opType byte

const (
	opPut    opType = 1
	opDelete opType = 2
)

// WalRecord is one logical entry in the log.
type WalRecord struct {
	op    opType
	key   string
	value string // empty/unused for opDelete
}

// WAL wraps an append-only file used for durability.
type WAL struct {
	file *os.File
}

func OpenWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

// Append writes one record to the log and fsyncs before returning.
//
// The fsync is the whole point: it forces the OS to actually flush the
// write to disk (not just to a page cache buffer) before we tell the
// caller "this write is durable". This is also the single biggest
// throughput bottleneck in the whole system - every real storage engine
// spends a lot of effort batching/grouping fsyncs (group commit) to
// amortize this cost. We do the naive thing here on purpose so you can
// feel the cost: try commenting out the Sync() call and benchmark it.
func (w *WAL) Append(rec WalRecord) error {
	buf := encodeRecord(rec)
	if _, err := w.file.Write(buf); err != nil {
		return err
	}
	return w.file.Sync()
}

func (w *WAL) AppendNoSync(rec WalRecord) error {
	buf := encodeRecord(rec)
	if _, err := w.file.Write(buf); err != nil {
		return err
	}
	return nil
}

func encodeRecord(rec WalRecord) []byte {
	keyBytes := []byte(rec.key)
	valBytes := []byte(rec.value)

	// body = op(1) + keyLen(4) + key + valLen(4) + val
	bodyLen := 1 + 4 + len(keyBytes) + 4 + len(valBytes)
	body := make([]byte, bodyLen)

	off := 0
	body[off] = byte(rec.op)
	off++
	binary.BigEndian.PutUint32(body[off:], uint32(len(keyBytes)))
	off += 4
	copy(body[off:], keyBytes)
	off += len(keyBytes)
	binary.BigEndian.PutUint32(body[off:], uint32(len(valBytes)))
	off += 4
	copy(body[off:], valBytes)

	checksum := crc32.ChecksumIEEE(body)

	// full record = totalLen(4) + body + crc32(4)
	out := make([]byte, 4+len(body)+4)
	binary.BigEndian.PutUint32(out[0:4], uint32(len(body)))
	copy(out[4:], body)
	binary.BigEndian.PutUint32(out[4+len(body):], checksum)
	return out
}

// Replay reads every valid record from the start of the log and calls
// fn for each one, in order. It stops (without error) the moment it
// hits a short read or a checksum mismatch, since that's exactly what
// a torn write from a mid-append crash looks like.
func Replay(path string, fn func(WalRecord) error) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(r, lenBuf); err != nil {
			// EOF here is the normal, expected end of the log.
			return nil
		}
		bodyLen := binary.BigEndian.Uint32(lenBuf)

		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(r, body); err != nil {
			// Partial body = torn write. Stop replay here.
			return nil
		}

		crcBuf := make([]byte, 4)
		if _, err := io.ReadFull(r, crcBuf); err != nil {
			return nil
		}
		wantCrc := binary.BigEndian.Uint32(crcBuf)
		gotCrc := crc32.ChecksumIEEE(body)
		if wantCrc != gotCrc {
			// Checksum mismatch = torn/corrupt write. Stop replay here,
			// discarding this and any later (unreachable) records.
			return nil
		}

		rec, err := decodeBody(body)
		if err != nil {
			return nil
		}

		if err := fn(rec); err != nil {
			return err
		}
	}
}

func decodeBody(body []byte) (WalRecord, error) {
	off := 0
	op := opType(body[off])
	off++

	keyLen := binary.BigEndian.Uint32(body[off:])
	off += 4
	key := string(body[off : off+int(keyLen)])
	off += int(keyLen)

	valLen := binary.BigEndian.Uint32(body[off:])
	off += 4
	val := string(body[off : off+int(valLen)])

	return WalRecord{op: op, key: key, value: val}, nil
}

func (w *WAL) Close() error {
	return w.file.Close()
}

// Truncate replaces the WAL file with an empty one. Used after
// compaction, once the current state has been safely snapshotted
// elsewhere and the old log entries are no longer needed for recovery.
func (w *WAL) Truncate() error {
	if err := w.file.Truncate(0); err != nil {
		return err
	}
	_, err := w.file.Seek(0, io.SeekStart)
	return err
}
