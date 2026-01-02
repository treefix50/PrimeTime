package server

// PlaybackState represents the stored playback position for a media item.
type PlaybackState struct {
	MediaID         string   `json:"mediaId"`
	PositionSeconds int64    `json:"positionSeconds"`
	DurationSeconds int64    `json:"durationSeconds"`
	UpdatedAt       int64    `json:"updatedAt"`
	LastPlayedAt    int64    `json:"lastPlayedAt"`
	PercentComplete *float64 `json:"percentComplete,omitempty"`
	ClientID        string   `json:"clientId,omitempty"`
}

// PlaybackEvent represents a client playback progress payload.
type PlaybackEvent struct {
	Event           string   `json:"event"`
	PositionSeconds int64    `json:"positionSeconds"`
	DurationSeconds int64    `json:"durationSeconds"`
	LastPlayedAt    int64    `json:"lastPlayedAt"`
	PercentComplete *float64 `json:"percentComplete,omitempty"`
	ClientID        string   `json:"clientId,omitempty"`
}
