package adobevideo

const (
	ChannelName              = "adobe-video"
	ProviderOptionsNamespace = "adobe_video"
	videoRequestContextKey   = "adobe_video_request"
	videoContentResolver     = "adobe-video-content"
	defaultAspectRatio       = "16:9"
	minDurationSeconds       = 4
	maxDurationSeconds       = 15
)

var ModelList = []string{
	"seedance_2.0_fast_480p",
	"seedance_2.0_fast_720p",
	"seedance_2.0_480p",
	"seedance_2.0_720p",
	"seedance_2.0_1080p",
}

var supportedModels = map[string]struct{}{
	"seedance_2.0_fast_480p": {},
	"seedance_2.0_fast_720p": {},
	"seedance_2.0_480p":      {},
	"seedance_2.0_720p":      {},
	"seedance_2.0_1080p":     {},
}
