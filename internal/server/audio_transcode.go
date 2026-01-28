package server

import (
	"fmt"
	"strings"
)

type audioTranscodeDecision struct {
	Codec        string
	BitrateKbps  int64
	DecisionNote string
}

func decideAudioTranscode(profile TranscodingProfile, selection AudioSelection) audioTranscodeDecision {
	priority := audioCodecPriority(profile)
	sourceCodec := strings.ToLower(strings.TrimSpace(selection.SourceCodec))
	if sourceCodec != "" {
		for _, codec := range priority {
			if codecMatches(sourceCodec, codec) {
				return audioTranscodeDecision{
					Codec:        "copy",
					DecisionNote: fmt.Sprintf("copy (source codec %s is supported)", sourceCodec),
				}
			}
		}
	}

	target := ""
	if len(priority) > 0 {
		target = priority[0]
	}
	if target == "" {
		target = strings.ToLower(strings.TrimSpace(profile.AudioCodec))
	}

	return audioTranscodeDecision{
		Codec:        target,
		BitrateKbps:  audioBitrateForCodec(target),
		DecisionNote: fmt.Sprintf("fallback to %s", target),
	}
}

func audioCodecPriority(profile TranscodingProfile) []string {
	priority := normalizeList(profile.SupportedAudioCodecs)
	if len(priority) == 0 {
		if value := strings.TrimSpace(profile.AudioCodec); value != "" {
			priority = []string{strings.ToLower(value)}
		}
	}
	return priority
}

func audioBitrateForCodec(codec string) int64 {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "opus":
		return 96
	case "aac":
		return 128
	default:
		return 128
	}
}
