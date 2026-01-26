package server

import (
	"encoding/xml"
	"os"
	"strings"
)

// Actor represents detailed actor information from NFO files
type Actor struct {
	Name   string `json:"name,omitempty"`
	Role   string `json:"role,omitempty"`
	Type   string `json:"type,omitempty"`
	TMDbID string `json:"tmdbId,omitempty"`
	TVDbID string `json:"tvdbId,omitempty"`
	IMDbID string `json:"imdbId,omitempty"`
}

// UniqueID represents external database IDs
type UniqueID struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// StreamDetails represents technical media information
type StreamDetails struct {
	Video    []VideoStream    `json:"video,omitempty"`
	Audio    []AudioStream    `json:"audio,omitempty"`
	Subtitle []SubtitleStream `json:"subtitle,omitempty"`
}

// VideoStream represents video track information
type VideoStream struct {
	Codec          string `json:"codec,omitempty"`
	Bitrate        string `json:"bitrate,omitempty"`
	Width          string `json:"width,omitempty"`
	Height         string `json:"height,omitempty"`
	Aspect         string `json:"aspect,omitempty"`
	AspectRatio    string `json:"aspectRatio,omitempty"`
	FrameRate      string `json:"frameRate,omitempty"`
	ScanType       string `json:"scanType,omitempty"`
	Duration       string `json:"duration,omitempty"`
	DurationInSecs string `json:"durationInSeconds,omitempty"`
}

// AudioStream represents audio track information
type AudioStream struct {
	Codec        string `json:"codec,omitempty"`
	Bitrate      string `json:"bitrate,omitempty"`
	Language     string `json:"language,omitempty"`
	Channels     string `json:"channels,omitempty"`
	SamplingRate string `json:"samplingRate,omitempty"`
}

// SubtitleStream represents subtitle track information
type SubtitleStream struct {
	Codec    string `json:"codec,omitempty"`
	Language string `json:"language,omitempty"`
}

// NFO represents a complete, parsed view of common Kodi-style .nfo files.
type NFO struct {
	Type        string   `json:"type"` // movie | tvshow | season | episode | musicvideo | person | unknown
	Title       string   `json:"title,omitempty"`
	Original    string   `json:"originalTitle,omitempty"`
	SortTitle   string   `json:"sortTitle,omitempty"`
	Plot        string   `json:"plot,omitempty"`
	Outline     string   `json:"outline,omitempty"`
	Tagline     string   `json:"tagline,omitempty"`
	Year        string   `json:"year,omitempty"`
	Rating      string   `json:"rating,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Season      string   `json:"season,omitempty"`
	Episode     string   `json:"episode,omitempty"`
	ShowTitle   string   `json:"showTitle,omitempty"`
	RawRootName string   `json:"rawRoot"`

	// Extended metadata fields
	Actors    []Actor  `json:"actors,omitempty"`
	Directors []string `json:"directors,omitempty"`
	Studios   []string `json:"studios,omitempty"`
	Runtime   string   `json:"runtime,omitempty"`
	IMDbID    string   `json:"imdbId,omitempty"`
	TMDbID    string   `json:"tmdbId,omitempty"`
	TVDbID    string   `json:"tvdbId,omitempty"`

	// Additional metadata
	MPAA        string     `json:"mpaa,omitempty"`
	Premiered   string     `json:"premiered,omitempty"`
	ReleaseDate string     `json:"releaseDate,omitempty"`
	Countries   []string   `json:"countries,omitempty"`
	Trailers    []string   `json:"trailers,omitempty"`
	UniqueIDs   []UniqueID `json:"uniqueIds,omitempty"`
	DateAdded   string     `json:"dateAdded,omitempty"`

	// Technical information
	StreamDetails *StreamDetails `json:"streamDetails,omitempty"`
}

// ParseNFOFile parses a Kodi-style XML .nfo file.
// If the file does not exist or cannot be parsed, an error is returned.
func ParseNFOFile(path string) (*NFO, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	root := detectRootName(data)

	switch root {

	case "movie":
		var m struct {
			XMLName       xml.Name `xml:"movie"`
			Title         string   `xml:"title"`
			OriginalTitle string   `xml:"originaltitle"`
			SortTitle     string   `xml:"sorttitle"`
			Plot          string   `xml:"plot"`
			Outline       string   `xml:"outline"`
			Tagline       string   `xml:"tagline"`
			Year          string   `xml:"year"`
			Rating        string   `xml:"rating"`
			Genres        []string `xml:"genre"`
			Actors        []struct {
				Name   string `xml:"name"`
				Role   string `xml:"role"`
				Type   string `xml:"type"`
				TMDbID string `xml:"tmdbid"`
				TVDbID string `xml:"tvdbid"`
				IMDbID string `xml:"imdbid"`
			} `xml:"actor"`
			Directors   []string `xml:"director"`
			Studios     []string `xml:"studio"`
			Runtime     string   `xml:"runtime"`
			IMDbID      string   `xml:"imdbid"`
			TMDbID      string   `xml:"tmdbid"`
			TVDbID      string   `xml:"tvdbid"`
			MPAA        string   `xml:"mpaa"`
			Premiered   string   `xml:"premiered"`
			ReleaseDate string   `xml:"releasedate"`
			Countries   []string `xml:"country"`
			Trailers    []string `xml:"trailer"`
			DateAdded   string   `xml:"dateadded"`
			UniqueIDs   []struct {
				Type  string `xml:"type,attr"`
				Value string `xml:",chardata"`
			} `xml:"uniqueid"`
			FileInfo struct {
				StreamDetails struct {
					Video []struct {
						Codec          string `xml:"codec"`
						Bitrate        string `xml:"bitrate"`
						Width          string `xml:"width"`
						Height         string `xml:"height"`
						Aspect         string `xml:"aspect"`
						AspectRatio    string `xml:"aspectratio"`
						FrameRate      string `xml:"framerate"`
						ScanType       string `xml:"scantype"`
						Duration       string `xml:"duration"`
						DurationInSecs string `xml:"durationinseconds"`
					} `xml:"video"`
					Audio []struct {
						Codec        string `xml:"codec"`
						Bitrate      string `xml:"bitrate"`
						Language     string `xml:"language"`
						Channels     string `xml:"channels"`
						SamplingRate string `xml:"samplingrate"`
					} `xml:"audio"`
					Subtitle []struct {
						Codec    string `xml:"codec"`
						Language string `xml:"language"`
					} `xml:"subtitle"`
				} `xml:"streamdetails"`
			} `xml:"fileinfo"`
		}

		if err := xml.Unmarshal(data, &m); err != nil {
			return nil, err
		}

		// Parse actors with full details
		actors := make([]Actor, 0, len(m.Actors))
		for _, a := range m.Actors {
			if strings.TrimSpace(a.Name) != "" {
				actors = append(actors, Actor{
					Name:   strings.TrimSpace(a.Name),
					Role:   strings.TrimSpace(a.Role),
					Type:   strings.TrimSpace(a.Type),
					TMDbID: strings.TrimSpace(a.TMDbID),
					TVDbID: strings.TrimSpace(a.TVDbID),
					IMDbID: strings.TrimSpace(a.IMDbID),
				})
			}
		}

		// Parse unique IDs
		uniqueIDs := make([]UniqueID, 0, len(m.UniqueIDs))
		for _, uid := range m.UniqueIDs {
			if strings.TrimSpace(uid.Type) != "" && strings.TrimSpace(uid.Value) != "" {
				uniqueIDs = append(uniqueIDs, UniqueID{
					Type:  strings.TrimSpace(uid.Type),
					Value: strings.TrimSpace(uid.Value),
				})
			}
		}

		// Parse stream details
		var streamDetails *StreamDetails
		if len(m.FileInfo.StreamDetails.Video) > 0 || len(m.FileInfo.StreamDetails.Audio) > 0 || len(m.FileInfo.StreamDetails.Subtitle) > 0 {
			streamDetails = &StreamDetails{}

			// Video streams
			for _, v := range m.FileInfo.StreamDetails.Video {
				streamDetails.Video = append(streamDetails.Video, VideoStream{
					Codec:          strings.TrimSpace(v.Codec),
					Bitrate:        strings.TrimSpace(v.Bitrate),
					Width:          strings.TrimSpace(v.Width),
					Height:         strings.TrimSpace(v.Height),
					Aspect:         strings.TrimSpace(v.Aspect),
					AspectRatio:    strings.TrimSpace(v.AspectRatio),
					FrameRate:      strings.TrimSpace(v.FrameRate),
					ScanType:       strings.TrimSpace(v.ScanType),
					Duration:       strings.TrimSpace(v.Duration),
					DurationInSecs: strings.TrimSpace(v.DurationInSecs),
				})
			}

			// Audio streams
			for _, a := range m.FileInfo.StreamDetails.Audio {
				streamDetails.Audio = append(streamDetails.Audio, AudioStream{
					Codec:        strings.TrimSpace(a.Codec),
					Bitrate:      strings.TrimSpace(a.Bitrate),
					Language:     strings.TrimSpace(a.Language),
					Channels:     strings.TrimSpace(a.Channels),
					SamplingRate: strings.TrimSpace(a.SamplingRate),
				})
			}

			// Subtitle streams
			for _, s := range m.FileInfo.StreamDetails.Subtitle {
				streamDetails.Subtitle = append(streamDetails.Subtitle, SubtitleStream{
					Codec:    strings.TrimSpace(s.Codec),
					Language: strings.TrimSpace(s.Language),
				})
			}
		}

		return &NFO{
			Type:          "movie",
			Title:         strings.TrimSpace(m.Title),
			Original:      strings.TrimSpace(m.OriginalTitle),
			SortTitle:     strings.TrimSpace(m.SortTitle),
			Plot:          strings.TrimSpace(m.Plot),
			Outline:       strings.TrimSpace(m.Outline),
			Tagline:       strings.TrimSpace(m.Tagline),
			Year:          strings.TrimSpace(m.Year),
			Rating:        strings.TrimSpace(m.Rating),
			Genres:        trimAll(m.Genres),
			Actors:        actors,
			Directors:     trimAll(m.Directors),
			Studios:       trimAll(m.Studios),
			Runtime:       strings.TrimSpace(m.Runtime),
			IMDbID:        strings.TrimSpace(m.IMDbID),
			TMDbID:        strings.TrimSpace(m.TMDbID),
			TVDbID:        strings.TrimSpace(m.TVDbID),
			MPAA:          strings.TrimSpace(m.MPAA),
			Premiered:     strings.TrimSpace(m.Premiered),
			ReleaseDate:   strings.TrimSpace(m.ReleaseDate),
			Countries:     trimAll(m.Countries),
			Trailers:      trimAll(m.Trailers),
			UniqueIDs:     uniqueIDs,
			DateAdded:     strings.TrimSpace(m.DateAdded),
			StreamDetails: streamDetails,
			RawRootName:   root,
		}, nil

	case "tvshow":
		var t struct {
			XMLName   xml.Name `xml:"tvshow"`
			Title     string   `xml:"title"`
			Original  string   `xml:"originaltitle"`
			SortTitle string   `xml:"sorttitle"`
			Plot      string   `xml:"plot"`
			Outline   string   `xml:"outline"`
			Tagline   string   `xml:"tagline"`
			Genres    []string `xml:"genre"`
			Actors    []struct {
				Name   string `xml:"name"`
				Role   string `xml:"role"`
				Type   string `xml:"type"`
				TMDbID string `xml:"tmdbid"`
				TVDbID string `xml:"tvdbid"`
				IMDbID string `xml:"imdbid"`
			} `xml:"actor"`
			Studios   []string `xml:"studio"`
			Runtime   string   `xml:"runtime"`
			IMDbID    string   `xml:"imdbid"`
			TMDbID    string   `xml:"tmdbid"`
			TVDbID    string   `xml:"tvdbid"`
			MPAA      string   `xml:"mpaa"`
			Premiered string   `xml:"premiered"`
			Year      string   `xml:"year"`
			Countries []string `xml:"country"`
			Trailers  []string `xml:"trailer"`
			DateAdded string   `xml:"dateadded"`
			UniqueIDs []struct {
				Type  string `xml:"type,attr"`
				Value string `xml:",chardata"`
			} `xml:"uniqueid"`
		}

		if err := xml.Unmarshal(data, &t); err != nil {
			return nil, err
		}

		// Parse actors with full details
		actors := make([]Actor, 0, len(t.Actors))
		for _, a := range t.Actors {
			if strings.TrimSpace(a.Name) != "" {
				actors = append(actors, Actor{
					Name:   strings.TrimSpace(a.Name),
					Role:   strings.TrimSpace(a.Role),
					Type:   strings.TrimSpace(a.Type),
					TMDbID: strings.TrimSpace(a.TMDbID),
					TVDbID: strings.TrimSpace(a.TVDbID),
					IMDbID: strings.TrimSpace(a.IMDbID),
				})
			}
		}

		// Parse unique IDs
		uniqueIDs := make([]UniqueID, 0, len(t.UniqueIDs))
		for _, uid := range t.UniqueIDs {
			if strings.TrimSpace(uid.Type) != "" && strings.TrimSpace(uid.Value) != "" {
				uniqueIDs = append(uniqueIDs, UniqueID{
					Type:  strings.TrimSpace(uid.Type),
					Value: strings.TrimSpace(uid.Value),
				})
			}
		}

		return &NFO{
			Type:        "tvshow",
			Title:       strings.TrimSpace(t.Title),
			Original:    strings.TrimSpace(t.Original),
			SortTitle:   strings.TrimSpace(t.SortTitle),
			Plot:        strings.TrimSpace(t.Plot),
			Outline:     strings.TrimSpace(t.Outline),
			Tagline:     strings.TrimSpace(t.Tagline),
			Year:        strings.TrimSpace(t.Year),
			Genres:      trimAll(t.Genres),
			Actors:      actors,
			Studios:     trimAll(t.Studios),
			Runtime:     strings.TrimSpace(t.Runtime),
			IMDbID:      strings.TrimSpace(t.IMDbID),
			TMDbID:      strings.TrimSpace(t.TMDbID),
			TVDbID:      strings.TrimSpace(t.TVDbID),
			MPAA:        strings.TrimSpace(t.MPAA),
			Premiered:   strings.TrimSpace(t.Premiered),
			Countries:   trimAll(t.Countries),
			Trailers:    trimAll(t.Trailers),
			UniqueIDs:   uniqueIDs,
			DateAdded:   strings.TrimSpace(t.DateAdded),
			RawRootName: root,
		}, nil

	case "episodedetails":
		var e struct {
			XMLName   xml.Name `xml:"episodedetails"`
			Title     string   `xml:"title"`
			Plot      string   `xml:"plot"`
			Outline   string   `xml:"outline"`
			Season    string   `xml:"season"`
			Episode   string   `xml:"episode"`
			ShowTitle string   `xml:"showtitle"`
			Rating    string   `xml:"rating"`
			Year      string   `xml:"year"`
			IMDbID    string   `xml:"imdbid"`
			TMDbID    string   `xml:"tmdbid"`
			TVDbID    string   `xml:"tvdbid"`
			Actors    []struct {
				Name   string `xml:"name"`
				Role   string `xml:"role"`
				Type   string `xml:"type"`
				TMDbID string `xml:"tmdbid"`
				TVDbID string `xml:"tvdbid"`
				IMDbID string `xml:"imdbid"`
			} `xml:"actor"`
			Directors []string `xml:"director"`
			Runtime   string   `xml:"runtime"`
			MPAA      string   `xml:"mpaa"`
			Premiered string   `xml:"aired"`
			DateAdded string   `xml:"dateadded"`
			UniqueIDs []struct {
				Type  string `xml:"type,attr"`
				Value string `xml:",chardata"`
			} `xml:"uniqueid"`
			FileInfo struct {
				StreamDetails struct {
					Video []struct {
						Codec          string `xml:"codec"`
						Bitrate        string `xml:"bitrate"`
						Width          string `xml:"width"`
						Height         string `xml:"height"`
						Aspect         string `xml:"aspect"`
						AspectRatio    string `xml:"aspectratio"`
						FrameRate      string `xml:"framerate"`
						ScanType       string `xml:"scantype"`
						Duration       string `xml:"duration"`
						DurationInSecs string `xml:"durationinseconds"`
					} `xml:"video"`
					Audio []struct {
						Codec        string `xml:"codec"`
						Bitrate      string `xml:"bitrate"`
						Language     string `xml:"language"`
						Channels     string `xml:"channels"`
						SamplingRate string `xml:"samplingrate"`
					} `xml:"audio"`
					Subtitle []struct {
						Codec    string `xml:"codec"`
						Language string `xml:"language"`
					} `xml:"subtitle"`
				} `xml:"streamdetails"`
			} `xml:"fileinfo"`
		}

		if err := xml.Unmarshal(data, &e); err != nil {
			return nil, err
		}

		// Parse actors with full details
		actors := make([]Actor, 0, len(e.Actors))
		for _, a := range e.Actors {
			if strings.TrimSpace(a.Name) != "" {
				actors = append(actors, Actor{
					Name:   strings.TrimSpace(a.Name),
					Role:   strings.TrimSpace(a.Role),
					Type:   strings.TrimSpace(a.Type),
					TMDbID: strings.TrimSpace(a.TMDbID),
					TVDbID: strings.TrimSpace(a.TVDbID),
					IMDbID: strings.TrimSpace(a.IMDbID),
				})
			}
		}

		// Parse unique IDs
		uniqueIDs := make([]UniqueID, 0, len(e.UniqueIDs))
		for _, uid := range e.UniqueIDs {
			if strings.TrimSpace(uid.Type) != "" && strings.TrimSpace(uid.Value) != "" {
				uniqueIDs = append(uniqueIDs, UniqueID{
					Type:  strings.TrimSpace(uid.Type),
					Value: strings.TrimSpace(uid.Value),
				})
			}
		}

		// Parse stream details
		var streamDetails *StreamDetails
		if len(e.FileInfo.StreamDetails.Video) > 0 || len(e.FileInfo.StreamDetails.Audio) > 0 || len(e.FileInfo.StreamDetails.Subtitle) > 0 {
			streamDetails = &StreamDetails{}

			// Video streams
			for _, v := range e.FileInfo.StreamDetails.Video {
				streamDetails.Video = append(streamDetails.Video, VideoStream{
					Codec:          strings.TrimSpace(v.Codec),
					Bitrate:        strings.TrimSpace(v.Bitrate),
					Width:          strings.TrimSpace(v.Width),
					Height:         strings.TrimSpace(v.Height),
					Aspect:         strings.TrimSpace(v.Aspect),
					AspectRatio:    strings.TrimSpace(v.AspectRatio),
					FrameRate:      strings.TrimSpace(v.FrameRate),
					ScanType:       strings.TrimSpace(v.ScanType),
					Duration:       strings.TrimSpace(v.Duration),
					DurationInSecs: strings.TrimSpace(v.DurationInSecs),
				})
			}

			// Audio streams
			for _, a := range e.FileInfo.StreamDetails.Audio {
				streamDetails.Audio = append(streamDetails.Audio, AudioStream{
					Codec:        strings.TrimSpace(a.Codec),
					Bitrate:      strings.TrimSpace(a.Bitrate),
					Language:     strings.TrimSpace(a.Language),
					Channels:     strings.TrimSpace(a.Channels),
					SamplingRate: strings.TrimSpace(a.SamplingRate),
				})
			}

			// Subtitle streams
			for _, s := range e.FileInfo.StreamDetails.Subtitle {
				streamDetails.Subtitle = append(streamDetails.Subtitle, SubtitleStream{
					Codec:    strings.TrimSpace(s.Codec),
					Language: strings.TrimSpace(s.Language),
				})
			}
		}

		return &NFO{
			Type:          "episode",
			Title:         strings.TrimSpace(e.Title),
			Plot:          strings.TrimSpace(e.Plot),
			Outline:       strings.TrimSpace(e.Outline),
			Season:        strings.TrimSpace(e.Season),
			Episode:       strings.TrimSpace(e.Episode),
			ShowTitle:     strings.TrimSpace(e.ShowTitle),
			Rating:        strings.TrimSpace(e.Rating),
			Year:          strings.TrimSpace(e.Year),
			IMDbID:        strings.TrimSpace(e.IMDbID),
			TMDbID:        strings.TrimSpace(e.TMDbID),
			TVDbID:        strings.TrimSpace(e.TVDbID),
			Actors:        actors,
			Directors:     trimAll(e.Directors),
			Runtime:       strings.TrimSpace(e.Runtime),
			MPAA:          strings.TrimSpace(e.MPAA),
			Premiered:     strings.TrimSpace(e.Premiered),
			UniqueIDs:     uniqueIDs,
			DateAdded:     strings.TrimSpace(e.DateAdded),
			StreamDetails: streamDetails,
			RawRootName:   root,
		}, nil

	case "episode":
		var e struct {
			XMLName   xml.Name `xml:"episode"`
			Title     string   `xml:"title"`
			Plot      string   `xml:"plot"`
			Outline   string   `xml:"outline"`
			Season    string   `xml:"season"`
			Episode   string   `xml:"episode"`
			ShowTitle string   `xml:"showtitle"`
			Rating    string   `xml:"rating"`
			Year      string   `xml:"year"`
			IMDbID    string   `xml:"imdbid"`
			TMDbID    string   `xml:"tmdbid"`
			TVDbID    string   `xml:"tvdbid"`
			Actors    []struct {
				Name   string `xml:"name"`
				Role   string `xml:"role"`
				Type   string `xml:"type"`
				TMDbID string `xml:"tmdbid"`
				TVDbID string `xml:"tvdbid"`
				IMDbID string `xml:"imdbid"`
			} `xml:"actor"`
			Directors []string `xml:"director"`
			Runtime   string   `xml:"runtime"`
			MPAA      string   `xml:"mpaa"`
			Premiered string   `xml:"aired"`
			DateAdded string   `xml:"dateadded"`
			UniqueIDs []struct {
				Type  string `xml:"type,attr"`
				Value string `xml:",chardata"`
			} `xml:"uniqueid"`
			FileInfo struct {
				StreamDetails struct {
					Video []struct {
						Codec          string `xml:"codec"`
						Bitrate        string `xml:"bitrate"`
						Width          string `xml:"width"`
						Height         string `xml:"height"`
						Aspect         string `xml:"aspect"`
						AspectRatio    string `xml:"aspectratio"`
						FrameRate      string `xml:"framerate"`
						ScanType       string `xml:"scantype"`
						Duration       string `xml:"duration"`
						DurationInSecs string `xml:"durationinseconds"`
					} `xml:"video"`
					Audio []struct {
						Codec        string `xml:"codec"`
						Bitrate      string `xml:"bitrate"`
						Language     string `xml:"language"`
						Channels     string `xml:"channels"`
						SamplingRate string `xml:"samplingrate"`
					} `xml:"audio"`
					Subtitle []struct {
						Codec    string `xml:"codec"`
						Language string `xml:"language"`
					} `xml:"subtitle"`
				} `xml:"streamdetails"`
			} `xml:"fileinfo"`
		}

		if err := xml.Unmarshal(data, &e); err != nil {
			return nil, err
		}

		actors := make([]Actor, 0, len(e.Actors))
		for _, a := range e.Actors {
			if strings.TrimSpace(a.Name) != "" {
				actors = append(actors, Actor{
					Name:   strings.TrimSpace(a.Name),
					Role:   strings.TrimSpace(a.Role),
					Type:   strings.TrimSpace(a.Type),
					TMDbID: strings.TrimSpace(a.TMDbID),
					TVDbID: strings.TrimSpace(a.TVDbID),
					IMDbID: strings.TrimSpace(a.IMDbID),
				})
			}
		}

		uniqueIDs := make([]UniqueID, 0, len(e.UniqueIDs))
		for _, uid := range e.UniqueIDs {
			if strings.TrimSpace(uid.Type) != "" && strings.TrimSpace(uid.Value) != "" {
				uniqueIDs = append(uniqueIDs, UniqueID{
					Type:  strings.TrimSpace(uid.Type),
					Value: strings.TrimSpace(uid.Value),
				})
			}
		}

		var streamDetails *StreamDetails
		if len(e.FileInfo.StreamDetails.Video) > 0 || len(e.FileInfo.StreamDetails.Audio) > 0 || len(e.FileInfo.StreamDetails.Subtitle) > 0 {
			streamDetails = &StreamDetails{}

			for _, v := range e.FileInfo.StreamDetails.Video {
				streamDetails.Video = append(streamDetails.Video, VideoStream{
					Codec:          strings.TrimSpace(v.Codec),
					Bitrate:        strings.TrimSpace(v.Bitrate),
					Width:          strings.TrimSpace(v.Width),
					Height:         strings.TrimSpace(v.Height),
					Aspect:         strings.TrimSpace(v.Aspect),
					AspectRatio:    strings.TrimSpace(v.AspectRatio),
					FrameRate:      strings.TrimSpace(v.FrameRate),
					ScanType:       strings.TrimSpace(v.ScanType),
					Duration:       strings.TrimSpace(v.Duration),
					DurationInSecs: strings.TrimSpace(v.DurationInSecs),
				})
			}

			for _, a := range e.FileInfo.StreamDetails.Audio {
				streamDetails.Audio = append(streamDetails.Audio, AudioStream{
					Codec:        strings.TrimSpace(a.Codec),
					Bitrate:      strings.TrimSpace(a.Bitrate),
					Language:     strings.TrimSpace(a.Language),
					Channels:     strings.TrimSpace(a.Channels),
					SamplingRate: strings.TrimSpace(a.SamplingRate),
				})
			}

			for _, s := range e.FileInfo.StreamDetails.Subtitle {
				streamDetails.Subtitle = append(streamDetails.Subtitle, SubtitleStream{
					Codec:    strings.TrimSpace(s.Codec),
					Language: strings.TrimSpace(s.Language),
				})
			}
		}

		return &NFO{
			Type:          "episode",
			Title:         strings.TrimSpace(e.Title),
			Plot:          strings.TrimSpace(e.Plot),
			Outline:       strings.TrimSpace(e.Outline),
			Season:        strings.TrimSpace(e.Season),
			Episode:       strings.TrimSpace(e.Episode),
			ShowTitle:     strings.TrimSpace(e.ShowTitle),
			Rating:        strings.TrimSpace(e.Rating),
			Year:          strings.TrimSpace(e.Year),
			IMDbID:        strings.TrimSpace(e.IMDbID),
			TMDbID:        strings.TrimSpace(e.TMDbID),
			TVDbID:        strings.TrimSpace(e.TVDbID),
			Actors:        actors,
			Directors:     trimAll(e.Directors),
			Runtime:       strings.TrimSpace(e.Runtime),
			MPAA:          strings.TrimSpace(e.MPAA),
			Premiered:     strings.TrimSpace(e.Premiered),
			UniqueIDs:     uniqueIDs,
			DateAdded:     strings.TrimSpace(e.DateAdded),
			StreamDetails: streamDetails,
			RawRootName:   root,
		}, nil

	case "season":
		var s struct {
			XMLName     xml.Name `xml:"season"`
			Title       string   `xml:"title"`
			SortTitle   string   `xml:"sorttitle"`
			Plot        string   `xml:"plot"`
			Outline     string   `xml:"outline"`
			Season      string   `xml:"seasonnumber"`
			Year        string   `xml:"year"`
			Premiered   string   `xml:"premiered"`
			ReleaseDate string   `xml:"releasedate"`
			DateAdded   string   `xml:"dateadded"`
			TVDbID      string   `xml:"tvdbid"`
			Countries   []string `xml:"country"`
			Studios     []string `xml:"studio"`
			Actors      []struct {
				Name   string `xml:"name"`
				Role   string `xml:"role"`
				Type   string `xml:"type"`
				TMDbID string `xml:"tmdbid"`
				TVDbID string `xml:"tvdbid"`
				IMDbID string `xml:"imdbid"`
			} `xml:"actor"`
			UniqueIDs []struct {
				Type  string `xml:"type,attr"`
				Value string `xml:",chardata"`
			} `xml:"uniqueid"`
		}

		if err := xml.Unmarshal(data, &s); err != nil {
			return nil, err
		}

		actors := make([]Actor, 0, len(s.Actors))
		for _, a := range s.Actors {
			if strings.TrimSpace(a.Name) != "" {
				actors = append(actors, Actor{
					Name:   strings.TrimSpace(a.Name),
					Role:   strings.TrimSpace(a.Role),
					Type:   strings.TrimSpace(a.Type),
					TMDbID: strings.TrimSpace(a.TMDbID),
					TVDbID: strings.TrimSpace(a.TVDbID),
					IMDbID: strings.TrimSpace(a.IMDbID),
				})
			}
		}

		uniqueIDs := make([]UniqueID, 0, len(s.UniqueIDs))
		for _, uid := range s.UniqueIDs {
			if strings.TrimSpace(uid.Type) != "" && strings.TrimSpace(uid.Value) != "" {
				uniqueIDs = append(uniqueIDs, UniqueID{
					Type:  strings.TrimSpace(uid.Type),
					Value: strings.TrimSpace(uid.Value),
				})
			}
		}

		return &NFO{
			Type:        "season",
			Title:       strings.TrimSpace(s.Title),
			SortTitle:   strings.TrimSpace(s.SortTitle),
			Plot:        strings.TrimSpace(s.Plot),
			Outline:     strings.TrimSpace(s.Outline),
			Season:      strings.TrimSpace(s.Season),
			Year:        strings.TrimSpace(s.Year),
			Premiered:   strings.TrimSpace(s.Premiered),
			ReleaseDate: strings.TrimSpace(s.ReleaseDate),
			DateAdded:   strings.TrimSpace(s.DateAdded),
			TVDbID:      strings.TrimSpace(s.TVDbID),
			Countries:   trimAll(s.Countries),
			Studios:     trimAll(s.Studios),
			Actors:      actors,
			UniqueIDs:   uniqueIDs,
			RawRootName: root,
		}, nil

	case "musicvideo":
		var mv struct {
			XMLName xml.Name `xml:"musicvideo"`
			Title   string   `xml:"title"`
			Album   string   `xml:"album"`
			Artist  []string `xml:"artist"`
			Plot    string   `xml:"plot"`
			Year    string   `xml:"year"`
			Rating  string   `xml:"rating"`
			Genres  []string `xml:"genre"`
		}

		if err := xml.Unmarshal(data, &mv); err != nil {
			return nil, err
		}

		return &NFO{
			Type:        "musicvideo",
			Title:       strings.TrimSpace(mv.Title),
			Original:    strings.TrimSpace(mv.Album),
			ShowTitle:   strings.TrimSpace(strings.Join(trimAll(mv.Artist), ", ")),
			Plot:        strings.TrimSpace(mv.Plot),
			Year:        strings.TrimSpace(mv.Year),
			Rating:      strings.TrimSpace(mv.Rating),
			Genres:      trimAll(mv.Genres),
			RawRootName: root,
		}, nil

	case "person":
		var p struct {
			XMLName   xml.Name `xml:"person"`
			Name      string   `xml:"name"`
			SortName  string   `xml:"sortname"`
			Biography string   `xml:"biography"`
			Born      string   `xml:"born"`
			Year      string   `xml:"year"`
		}

		if err := xml.Unmarshal(data, &p); err != nil {
			return nil, err
		}

		return &NFO{
			Type:        "person",
			Title:       strings.TrimSpace(p.Name),
			Original:    strings.TrimSpace(p.SortName),
			Plot:        strings.TrimSpace(p.Biography),
			Year:        extractYear(p.Year, p.Born),
			RawRootName: root,
		}, nil

	default:
		// Unknown or unsupported root element
		return &NFO{
			Type:        "unknown",
			RawRootName: root,
		}, nil
	}
}

// detectRootName extracts the root XML element name from a .nfo file.
func detectRootName(data []byte) string {
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "\ufeff") {
		s = strings.TrimPrefix(s, "\ufeff")
		s = strings.TrimSpace(s)
	}

	// strip XML header
	if strings.HasPrefix(s, "<?xml") {
		if i := strings.Index(s, "?>"); i >= 0 {
			s = strings.TrimSpace(s[i+2:])
		}
	}

	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "<!--") {
			if i := strings.Index(s, "-->"); i >= 0 {
				s = s[i+3:]
				continue
			}
		}
		break
	}

	// expect <root ...>
	if strings.HasPrefix(s, "<") {
		s = s[1:]
	}

	end := strings.IndexAny(s, " >\n\r\t")
	if end == -1 {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(s[:end]))
}

// trimAll trims whitespace and removes empty strings.
func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func extractYear(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		for i := 0; i <= len(value)-4; i++ {
			if value[i] < '0' || value[i] > '9' {
				continue
			}
			year := value[i : i+4]
			if year[1] < '0' || year[1] > '9' {
				continue
			}
			if year[2] < '0' || year[2] > '9' {
				continue
			}
			if year[3] < '0' || year[3] > '9' {
				continue
			}
			return year
		}
	}
	return ""
}
