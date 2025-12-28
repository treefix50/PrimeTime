package server

// MediaStore defines the storage operations used for media metadata.
type MediaStore interface {
	SaveItems(items []MediaItem) error
	DeleteItems(ids []string) error
	GetAll() ([]MediaItem, error)
	GetByID(id string) (MediaItem, bool, error)
	SaveNFO(mediaID string, nfo *NFO) error
	DeleteNFO(mediaID string) error
	GetNFO(mediaID string) (*NFO, bool, error)
	UpsertPlaybackState(mediaID string, positionSeconds, durationSeconds int64, clientID string) error
	GetPlaybackState(mediaID, clientID string) (*PlaybackState, bool, error)
	DeletePlaybackState(mediaID, clientID string) error
}
