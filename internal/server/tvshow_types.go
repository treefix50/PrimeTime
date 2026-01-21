package server

import "time"

// TVShowGroup represents a TV show with aggregated episode information
type TVShowGroup struct {
	ShowTitle      string    `json:"showTitle"`
	EpisodeCount   int       `json:"episodeCount"`
	SeasonCount    int       `json:"seasonCount"`
	FirstSeason    int       `json:"firstSeason"`
	LastModified   time.Time `json:"lastModified"`
	Year           string    `json:"year,omitempty"`
	FirstEpisodeID string    `json:"firstEpisodeId"` // For poster lookup
}

// TVSeasonGroup represents a season within a TV show
type TVSeasonGroup struct {
	ShowTitle    string    `json:"showTitle"`
	SeasonNumber int       `json:"seasonNumber"`
	EpisodeCount int       `json:"episodeCount"`
	LastModified time.Time `json:"lastModified"`
	EpisodeIDs   []string  `json:"episodeIds"`
}
