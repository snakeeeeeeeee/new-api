package constant

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"
)

const (
	SunoActionMusic  = "MUSIC"
	SunoActionLyrics = "LYRICS"

	TaskActionGenerate          = "generate"
	TaskActionImageGeneration   = "imageGeneration"
	TaskActionImageEdit         = "imageEdit"
	TaskActionTextGenerate      = "textGenerate"
	TaskActionFirstTailGenerate = "firstTailGenerate"
	TaskActionReferenceGenerate = "referenceGenerate"
	TaskActionRemix             = "remixGenerate"
	TaskActionVideoGeneration   = "videoGeneration"
	TaskActionVideoEdit         = "videoEdit"
	TaskActionVideoExtension    = "videoExtension"
)

var SunoModel2Action = map[string]string{
	"suno_music":  SunoActionMusic,
	"suno_lyrics": SunoActionLyrics,
}

const (
	TaskAssetTypeImage = "image"
	TaskAssetTypeVideo = "video"
	TaskAssetTypeAudio = "audio"
	TaskAssetTypeOther = "other"
)

func TaskActionAssetType(action string) string {
	switch action {
	case TaskActionImageGeneration, TaskActionImageEdit:
		return TaskAssetTypeImage
	case TaskActionGenerate,
		TaskActionTextGenerate,
		TaskActionFirstTailGenerate,
		TaskActionReferenceGenerate,
		TaskActionRemix,
		TaskActionVideoGeneration,
		TaskActionVideoEdit,
		TaskActionVideoExtension:
		return TaskAssetTypeVideo
	case SunoActionMusic, SunoActionLyrics:
		return TaskAssetTypeAudio
	default:
		return TaskAssetTypeOther
	}
}

func TaskActionsByAssetType(assetType string) []string {
	switch assetType {
	case TaskAssetTypeImage:
		return []string{TaskActionImageGeneration, TaskActionImageEdit}
	case TaskAssetTypeVideo:
		return []string{
			TaskActionGenerate,
			TaskActionTextGenerate,
			TaskActionFirstTailGenerate,
			TaskActionReferenceGenerate,
			TaskActionRemix,
			TaskActionVideoGeneration,
			TaskActionVideoEdit,
			TaskActionVideoExtension,
		}
	case TaskAssetTypeAudio:
		return []string{SunoActionMusic, SunoActionLyrics}
	default:
		return nil
	}
}
