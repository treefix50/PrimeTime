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
	GetAllLimited(limit, offset int, sortBy, query string) ([]MediaItem, error)
	// Verbesserung 3: Erweiterte Suchfunktionalität
	GetAllLimitedWithFilters(limit, offset int, sortBy, query, genre, year, itemType, rating string) ([]MediaItem, error)
	GetByID(id string) (MediaItem, bool, error)
	GetIDByPath(path string) (string, bool, error)
	SaveNFO(mediaID string, nfo *NFO) error
	DeleteNFO(mediaID string) error
	GetNFO(mediaID string) (*NFO, bool, error)
	// Extended NFO methods for complete metadata support
	SaveNFOExtended(mediaID string, nfo *NFO) error
	GetNFOExtended(mediaID string) (*NFO, bool, error)
	DeleteNFOExtended(mediaID string) error
	UpsertPlaybackState(mediaID string, positionSeconds, durationSeconds int64, lastPlayedAt int64, percentComplete *float64, clientID string) error
	GetPlaybackState(mediaID, clientID string) (*PlaybackState, bool, error)
	DeletePlaybackState(mediaID, clientID string) error
	// Verbesserung 2: Batch-Operations für Playback-State
	GetAllPlaybackStates(clientID string, onlyUnfinished bool) ([]PlaybackState, error)
	// Verbesserung 4: Duplicate Detection
	GetDuplicates() ([]DuplicateGroup, error)
	// Verbesserung 5: Erweiterte Statistiken
	GetDetailedStats() (*DetailedStats, error)

	// Erweiterung 1: Watched/Unwatched Status
	MarkWatched(mediaID string, watchedAt time.Time) error
	UnmarkWatched(mediaID string) error
	IsWatched(mediaID string) (bool, error)
	GetWatchedItems(limit, offset int) ([]MediaItem, error)

	// Erweiterung 2: Favorites/Bookmarks
	AddFavorite(mediaID string, addedAt time.Time) error
	RemoveFavorite(mediaID string) error
	IsFavorite(mediaID string) (bool, error)
	GetFavorites(limit, offset int) ([]MediaItem, error)

	// Erweiterung 3: Recently Added
	GetRecentlyAdded(limit int, days int, itemType string) ([]MediaItem, error)

	// Erweiterung 4: Collections/Playlists
	CreateCollection(id, name, description string, createdAt time.Time) error
	GetCollections(limit, offset int) ([]Collection, error)
	GetCollection(id string) (*Collection, bool, error)
	UpdateCollection(id, name, description string, updatedAt time.Time) error
	DeleteCollection(id string) error
	AddItemToCollection(collectionID, mediaID string, position int, addedAt time.Time) error
	RemoveItemFromCollection(collectionID, mediaID string) error
	GetCollectionItems(collectionID string) ([]MediaItem, error)

	// Erweiterung 5: Poster/Thumbnail Support
	GetPosterPath(mediaID string) (string, bool, error)
	SetPosterPath(mediaID, posterPath string) error

	// Multi-Root Support
	GetItemsByRoots(rootIDs []string, limit, offset int, sortBy, query string) ([]MediaItem, error)

	// NFO-based filtering and TV Show grouping
	GetItemsByNFOType(nfoType string, limit, offset int, sortBy, query string) ([]MediaItem, error)
	GetTVShowsGrouped(limit, offset int, sortBy, query string) ([]TVShowGroup, error)
	GetSeasonsByShowTitle(showTitle string) ([]TVSeasonGroup, error)
	GetEpisodesByShowAndSeason(showTitle string, seasonNumber int) ([]MediaItem, error)

	// Verbesserung 1: Multi-User-Support (Media Users)
	CreateMediaUser(id, name string, createdAt time.Time) error
	GetMediaUser(id string) (*User, bool, error)
	GetMediaUserByName(name string) (*User, bool, error)
	GetAllUsers() ([]User, error)
	UpdateUserLastActive(id string, lastActive time.Time) error
	DeleteMediaUser(id string) error
	SetUserPreference(userID, key, value string) error
	GetUserPreference(userID, key string) (string, bool, error)
	GetAllUserPreferences(userID string) (map[string]string, error)

	// Verbesserung 2: Transkodierungs-Profile
	CreateTranscodingProfile(profile TranscodingProfile) error
	GetTranscodingProfile(id string) (*TranscodingProfile, bool, error)
	GetTranscodingProfileByName(name string) (*TranscodingProfile, bool, error)
	GetAllTranscodingProfiles() ([]TranscodingProfile, error)
	DeleteTranscodingProfile(id string) error
	SaveTranscodingCache(cache TranscodingCache) error
	GetTranscodingCache(mediaID, profileID string) (*TranscodingCache, bool, error)
	DeleteTranscodingCache(id string) error
	CleanOldTranscodingCache(olderThan time.Time) error

	// Verbesserung 3: Serien-Verwaltung (TV Shows)
	CreateTVShow(show TVShow) error
	GetTVShow(id string) (*TVShow, bool, error)
	GetAllTVShows(limit, offset int) ([]TVShow, error)
	UpdateTVShow(show TVShow) error
	DeleteTVShow(id string) error
	CreateSeason(season Season) error
	GetSeason(id string) (*Season, bool, error)
	GetSeasonsByShow(showID string) ([]Season, error)
	UpdateSeason(season Season) error
	DeleteSeason(id string) error
	CreateEpisode(episode Episode) error
	GetEpisode(id string) (*Episode, bool, error)
	GetEpisodesBySeason(seasonID string) ([]Episode, error)
	GetEpisodeByMediaID(mediaID string) (*Episode, bool, error)
	UpdateEpisode(episode Episode) error
	DeleteEpisode(id string) error
	GetNextUnwatchedEpisode(showID, userID string) (*Episode, bool, error)
	AutoGroupEpisodes() error
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

// Verbesserung 4: Duplicate Detection
type DuplicateGroup struct {
	StableKey string      `json:"stableKey"`
	Items     []MediaItem `json:"items"`
	Count     int         `json:"count"`
}

// Verbesserung 5: Erweiterte Statistiken
type DetailedStats struct {
	TotalItems      int            `json:"totalItems"`
	TotalSizeBytes  int64          `json:"totalSizeBytes"`
	ItemsByType     map[string]int `json:"itemsByType"`
	ItemsWithNFO    int            `json:"itemsWithNFO"`
	ItemsWithoutNFO int            `json:"itemsWithoutNFO"`
	TopWatchedItems []TopItem      `json:"topWatchedItems"`
	RecentScans     []ScanRun      `json:"recentScans"`
}

type TopItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	WatchCount int    `json:"watchCount"`
}

// Erweiterung 4: Collection type
type Collection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ItemCount   int       `json:"itemCount"`
}

// Verbesserung 1: User types
type User struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"createdAt"`
	LastActive time.Time `json:"lastActive,omitempty"`
}

// Verbesserung 2: Transcoding types
type TranscodingProfile struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	VideoCodec           string    `json:"videoCodec"`
	AudioCodec           string    `json:"audioCodec"`
	SupportedAudioCodecs []string  `json:"supportedAudioCodecs,omitempty"`
	MaxAudioChannels     int       `json:"maxAudioChannels,omitempty"`
	AudioLayout          string    `json:"audioLayout,omitempty"`
	AudioNormalization   string    `json:"audioNormalization,omitempty"`
	PreferredLanguages   []string  `json:"preferredLanguages,omitempty"`
	Resolution           string    `json:"resolution,omitempty"`
	MaxBitrate           int64     `json:"maxBitrate,omitempty"`
	Container            string    `json:"container"`
	CreatedAt            time.Time `json:"createdAt"`
}

type TranscodingCache struct {
	ID           string    `json:"id"`
	MediaID      string    `json:"mediaId"`
	ProfileID    string    `json:"profileId"`
	CachePath    string    `json:"cachePath"`
	CreatedAt    time.Time `json:"createdAt"`
	LastAccessed time.Time `json:"lastAccessed"`
	SizeBytes    int64     `json:"sizeBytes"`
}

// Verbesserung 3: TV Show types
type TVShow struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"originalTitle,omitempty"`
	Plot          string    `json:"plot,omitempty"`
	PosterPath    string    `json:"posterPath,omitempty"`
	Year          int       `json:"year,omitempty"`
	Genres        []string  `json:"genres,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SeasonCount   int       `json:"seasonCount"`
	EpisodeCount  int       `json:"episodeCount"`
}

type Season struct {
	ID           string    `json:"id"`
	ShowID       string    `json:"showId"`
	SeasonNumber int       `json:"seasonNumber"`
	Title        string    `json:"title,omitempty"`
	Plot         string    `json:"plot,omitempty"`
	PosterPath   string    `json:"posterPath,omitempty"`
	EpisodeCount int       `json:"episodeCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Episode struct {
	ID            string    `json:"id"`
	SeasonID      string    `json:"seasonId"`
	EpisodeNumber int       `json:"episodeNumber"`
	MediaID       string    `json:"mediaId"`
	Title         string    `json:"title,omitempty"`
	Plot          string    `json:"plot,omitempty"`
	AirDate       string    `json:"airDate,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}
