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
)

func seedanceCapability() videoCapability {
	return videoCapability{
		family: "seedance", minDuration: 4, maxDuration: 15,
		aspectRatios:        seedanceAspectRatios,
		referenceImageLimit: map[string]int{"frame": 2, "media": 9},
		maxReferenceVideos:  3, maxReferenceAudios: 3, maxReferences: 12,
	}
}
