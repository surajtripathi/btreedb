package btreestore

import "os"

type Page struct {
	file os.File
}

const DIR = ""
const FILE_PREFIX = ""

//func ReadPage(pageID string) ([]byte, error) {
//
//}
//
//func WritePage(bytes []byte, pageID string) error {
//
//}
