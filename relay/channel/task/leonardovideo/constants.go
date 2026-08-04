package leonardovideo

const (
	ChannelName              = "leonardo-video"
	ProviderOptionsNamespace = "leonardo_video"
	videoRequestContextKey   = "leonardo_video_request"
	videoContentResolver     = "leonardo-video-content"
	defaultAspectRatio       = "16:9"
	maxVideoPromptRunes      = 1200
)

var ModelList = []string{
	"seedance-2.0-fast-480p",
	"seedance-2.0-fast-720p",
	"seedance-2.0-480p",
	"seedance-2.0-720p",
	"seedance-2.0-1080p",
	"minimax-h3-1440p",
}

var supportedModels = map[string]struct{}{
	"seedance-2.0-fast-480p": {},
	"seedance-2.0-fast-720p": {},
	"seedance-2.0-480p":      {},
	"seedance-2.0-720p":      {},
	"seedance-2.0-1080p":     {},
	"minimax-h3-1440p":       {},
}

var supportedAspectRatios = map[string]struct{}{
	"21:9": {},
	"16:9": {},
	"4:3":  {},
	"1:1":  {},
	"3:4":  {},
	"9:16": {},
}

type referenceMedia struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
}

type normalizedRequest struct {
	Model           string
	Prompt          string
	Duration        int
	AspectRatio     string
	GenerateAudio   *bool
	ReferenceMode   string
	ReferenceImages []referenceMedia
	ReferenceVideos []referenceMedia
	ReferenceAudios []referenceMedia
}

type upstreamRequest struct {
	Model           string           `json:"model"`
	Prompt          string           `json:"prompt"`
	Duration        int              `json:"duration"`
	AspectRatio     string           `json:"aspect_ratio"`
	GenerateAudio   *bool            `json:"generate_audio,omitempty"`
	Public          bool             `json:"public"`
	Seed            *int             `json:"seed,omitempty"`
	ReferenceMode   string           `json:"reference_mode,omitempty"`
	ImageReferences []referenceMedia `json:"image_references,omitempty"`
	VideoReferences []referenceMedia `json:"video_references,omitempty"`
	AudioReferences []referenceMedia `json:"audio_references,omitempty"`
}

type responseError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type responsePayload struct {
	ID       string         `json:"id"`
	TaskID   string         `json:"task_id"`
	Status   string         `json:"status"`
	Progress int            `json:"progress"`
	Duration int            `json:"duration"`
	VideoURL string         `json:"video_url,omitempty"`
	Error    *responseError `json:"error,omitempty"`
}
