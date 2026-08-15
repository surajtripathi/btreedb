package btreestore

import (
	"errors"
	"os"
)

type Pager struct {
	file        *os.File
	pageCounter uint64
}

var PageSizeMismatchError = errors.New("page size mismatch")
var InvalidPageIDError = errors.New("page id can not be greater than the page counter")

const DIR = "store"
const FilePrefix = "btree"

func newPager(path string) *Pager {
	err := os.MkdirAll(DIR, 0o755)
	if err != nil {
		panic(err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	stat, err := file.Stat()
	if err != nil {
		panic(err)
	}
	fileSize := stat.Size()
	return &Pager{
		file:        file,
		pageCounter: uint64(fileSize / pageSize),
	}
}

func (p *Pager) readPage(pageID uint64) ([]byte, error) {
	if pageID >= p.pageCounter {
		return nil, InvalidPageIDError
	}
	pageOffset := pageID * pageSize
	buff := make([]byte, pageSize)
	_, err := p.file.ReadAt(buff, int64(pageOffset))
	if err != nil {
		return nil, err
	}
	return buff, nil
}

func (p *Pager) writePage(bytes []byte, pageID uint64) error {
	if len(bytes) != pageSize {
		return PageSizeMismatchError
	}
	_, err := p.file.WriteAt(bytes, int64(pageID*pageSize))
	if err != nil {
		return err
	}
	return nil
}

func (p *Pager) allocatePage() uint64 {
	pageID := p.pageCounter
	p.pageCounter += 1
	return pageID
}

func (p *Pager) Close() error {
	return p.file.Close()
}

func (p *Pager) Sync() error {
	return p.file.Sync()
}
