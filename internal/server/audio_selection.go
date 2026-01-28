package server

import (
	"strconv"
	"strings"
)

type AudioSelection struct {
	TrackIndex        int
	PreferredLanguage string
	SourceCodec       string
}

func selectAudioSelection(profile TranscodingProfile, item MediaItem, store MediaStore) AudioSelection {
	selection := AudioSelection{
		TrackIndex:        -1,
		PreferredLanguage: firstNonEmpty(profile.PreferredLanguages),
	}

	var nfo *NFO
	if store != nil {
		if stored, ok, err := store.GetNFOExtended(item.ID); err == nil && ok {
			nfo = stored
		}
	}
	if nfo == nil && item.NFOPath != "" {
		if parsed, err := ParseNFOFile(item.NFOPath); err == nil {
			nfo = parsed
		}
	}
	if nfo == nil || nfo.StreamDetails == nil || len(nfo.StreamDetails.Audio) == 0 {
		return selection
	}

	index, language := chooseAudioStream(profile, nfo.StreamDetails.Audio)
	if index >= 0 {
		selection.TrackIndex = index
		if language != "" {
			selection.PreferredLanguage = language
		}
		if index < len(nfo.StreamDetails.Audio) {
			selection.SourceCodec = nfo.StreamDetails.Audio[index].Codec
		}
	}

	return selection
}

func chooseAudioStream(profile TranscodingProfile, streams []AudioStream) (int, string) {
	preferredLanguages := normalizeList(profile.PreferredLanguages)
	supportedCodecs := normalizeList(profile.SupportedAudioCodecs)
	maxChannels := profile.MaxAudioChannels

	if len(preferredLanguages) > 0 {
		for _, lang := range preferredLanguages {
			if idx := findMatchingStream(streams, lang, supportedCodecs, maxChannels); idx >= 0 {
				return idx, streams[idx].Language
			}
		}
	}

	if len(supportedCodecs) > 0 {
		if idx := findMatchingStream(streams, "", supportedCodecs, maxChannels); idx >= 0 {
			return idx, streams[idx].Language
		}
	}

	if maxChannels > 0 {
		for i, stream := range streams {
			if withinChannelLimit(stream, maxChannels) {
				return i, stream.Language
			}
		}
	}

	if len(streams) > 0 {
		return 0, streams[0].Language
	}

	return -1, ""
}

func findMatchingStream(streams []AudioStream, language string, codecs []string, maxChannels int) int {
	language = normalizeLanguage(language)
	if len(codecs) > 0 {
		for _, codec := range codecs {
			for i, stream := range streams {
				if !codecMatches(stream.Codec, codec) {
					continue
				}
				if language != "" && !languageMatches(stream.Language, language) {
					continue
				}
				if !withinChannelLimit(stream, maxChannels) {
					continue
				}
				return i
			}
		}
		return -1
	}

	for i, stream := range streams {
		if language != "" && !languageMatches(stream.Language, language) {
			continue
		}
		if !withinChannelLimit(stream, maxChannels) {
			continue
		}
		return i
	}
	return -1
}

func withinChannelLimit(stream AudioStream, maxChannels int) bool {
	if maxChannels <= 0 {
		return true
	}
	channels := parseChannelCount(stream.Channels)
	if channels == 0 {
		return true
	}
	return channels <= maxChannels
}

func parseChannelCount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if count, err := strconv.Atoi(value); err == nil {
		return count
	}
	var digits strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		} else if digits.Len() > 0 {
			break
		}
	}
	if digits.Len() == 0 {
		return 0
	}
	if count, err := strconv.Atoi(digits.String()); err == nil {
		return count
	}
	return 0
}

func normalizeList(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, strings.ToLower(value))
	}
	return normalized
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeLanguage(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func languageMatches(candidate, preferred string) bool {
	candidate = normalizeLanguage(candidate)
	preferred = normalizeLanguage(preferred)
	if candidate == "" || preferred == "" {
		return false
	}
	if candidate == preferred {
		return true
	}
	return strings.HasPrefix(candidate, preferred) || strings.HasPrefix(preferred, candidate)
}

func codecMatches(candidate, preferred string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	preferred = strings.ToLower(strings.TrimSpace(preferred))
	if candidate == "" || preferred == "" {
		return false
	}
	return candidate == preferred
}
