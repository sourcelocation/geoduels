package storage

type Store interface {
	CleanupStorage(batchSize int) (StorageCleanupResult, error)
}

type Service struct{ Store }

func NewService(store Store) *Service { return &Service{Store: store} }
