package server

// MediaStore defines the storage operations used for media metadata.
type MediaStore interface {
	SaveItems(items []MediaItem) error
	GetAll() ([]MediaItem, error)
	GetByID(id string) (MediaItem, bool, error)
	SaveNFO(mediaID string, nfo *NFO) error
	GetNFO(mediaID string) (*NFO, bool, error)
}
