package adobevideo

const (
	ChannelName              = "adobe-video"
	ProviderOptionsNamespace = "adobe_video"
	videoRequestContextKey   = "adobe_video_request"
	videoContentResolver     = "adobe-video-content"
	defaultAspectRatio       = "16:9"
	maxVideoPromptRunes      = 1200
)

type videoCapability struct {
	family                string
	minDuration           int
	maxDuration           int
	allowedDurations      map[int]struct{}
	aspectRatios          map[string]struct{}
	referenceImageLimit   map[string]int
	referenceImageMinimum map[string]int
	maxReferenceVideos    int
	maxReferenceAudios    int
	maxReferences         int
}

var (
	seedanceAspectRatios = map[string]struct{}{
		"21:9": {}, "16:9": {}, "4:3": {}, "1:1": {}, "3:4": {}, "9:16": {},
	}
	stableAspectRatios = map[string]struct{}{"16:9": {}, "9:16": {}}
	veoDurations       = map[int]struct{}{4: {}, 6: {}, 8: {}}

	ModelList = []string{
		"seedance_2.0_fast_480p",
		"seedance_2.0_fast_720p",
		"seedance_2.0_480p",
		"seedance_2.0_720p",
		"seedance_2.0_1080p",
		"kling_3.0_720p",
		"kling_3.0_1080p",
		"kling_3.0_omni_720p",
		"kling_3.0_omni_1080p",
		"veo_3.1_standard_720p",
		"veo_3.1_standard_1080p",
		"veo_3.1_fast_720p",
		"veo_3.1_fast_1080p",
	}

	supportedModels = map[string]videoCapability{
		"seedance_2.0_fast_480p": seedanceCapability(),
		"seedance_2.0_fast_720p": seedanceCapability(),
		"seedance_2.0_480p":      seedanceCapability(),
		"seedance_2.0_720p":      seedanceCapability(),
		"seedance_2.0_1080p":     seedanceCapability(),
		"kling_3.0_720p":         stableCapability("kling-3.0", 3, 15, nil, map[string]int{"frame": 2}, nil),
		"kling_3.0_1080p":        stableCapability("kling-3.0", 3, 15, nil, map[string]int{"frame": 2}, nil),
		"kling_3.0_omni_720p":    stableCapability("kling-3.0-omni", 3, 15, nil, map[string]int{"frame": 2, "images": 3}, nil),
		"kling_3.0_omni_1080p":   stableCapability("kling-3.0-omni", 3, 15, nil, map[string]int{"frame": 2, "images": 3}, nil),
		"veo_3.1_standard_720p":  stableCapability("veo-3.1-standard", 0, 0, veoDurations, map[string]int{"frame": 2, "images": 3}, nil),
		"veo_3.1_standard_1080p": stableCapability("veo-3.1-standard", 0, 0, veoDurations, map[string]int{"frame": 2, "images": 3}, nil),
		"veo_3.1_fast_720p":      stableCapability("veo-3.1-fast", 0, 0, veoDurations, map[string]int{"frame": 2}, nil),
		"veo_3.1_fast_1080p":     stableCapability("veo-3.1-fast", 0, 0, veoDurations, map[string]int{"frame": 2}, nil),
	}
	supportedModelNames = func() map[string]struct{} {
		models := make(map[string]struct{}, len(supportedModels))
		for model := range supportedModels {
			models[model] = struct{}{}
		}
		return models
	}()
)

func seedanceCapability() videoCapability {
	return videoCapability{
		family: "seedance", minDuration: 4, maxDuration: 15,
		aspectRatios:        seedanceAspectRatios,
		referenceImageLimit: map[string]int{"frame": 2, "media": 9},
		maxReferenceVideos:  3, maxReferenceAudios: 3, maxReferences: 12,
	}
}

func stableCapability(family string, minDuration, maxDuration int, durations map[int]struct{}, modes map[string]int, minimums map[string]int) videoCapability {
	return videoCapability{
		family: family, minDuration: minDuration, maxDuration: maxDuration,
		allowedDurations: durations, aspectRatios: stableAspectRatios,
		referenceImageLimit:   modes,
		referenceImageMinimum: minimums,
	}
}
