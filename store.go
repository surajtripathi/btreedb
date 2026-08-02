package btreestore

type Store struct {
}

func Open(walPath string) (*Store, error) {
	return &Store{}, nil
}

func (s *Store) Get(key string) (string, error) {
	return "", nil
}

func (s *Store) Put(key string, value string) error {
	return nil
}

func (s *Store) Delete(key string) error {
	return nil
}

func (s *Store) Close() error {
	return nil
}
