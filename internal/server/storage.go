package server

import "time"

// MediaStore defines the storage operations used for media metadata.
type MediaStore interface {
	ReadOnly() bool
	AddRoot(path, rootType string) (LibraryRoot, error)
	ListRoots() ([]LibraryRoot, error)
	RemoveRoot(id string) error
	StartScanRun(rootID string, startedAt time.Time) (ScanRun, error)
	FinishScanRun(id string, finishedAt time.Time) error
	FailScanRun(id string, finishedAt time.Time, errMsg string) error
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

type LibraryRoot struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
}

type ScanRun struct {
	ID         string    `json:"id"`
	RootID     string    `json:"rootId"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}
