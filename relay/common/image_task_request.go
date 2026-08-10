package common

import "github.com/QuantumNous/new-api/dto"

func NormalizedImageTaskToLegacy(request dto.ImageTaskCreateRequest) TaskSubmitReq {
	metadata := make(map[string]any, 8)
	metadata["operation"] = request.Operation
	metadata["result_data_format"] = "url"
	if len(request.ProviderOptions) > 0 {
		metadata["provider_options"] = request.ProviderOptions
	}
	legacy := TaskSubmitReq{
		Prompt:   request.Input.Prompt,
		Model:    request.Model,
		Mode:     request.Operation,
		Metadata: metadata,
	}
	for _, source := range request.Input.Images {
		legacy.Images = append(legacy.Images, source.URL)
	}
	if request.Input.Mask != nil {
		metadata["mask"] = request.Input.Mask.URL
	}
	if request.Output.Count != nil {
		legacy.N = request.Output.Count
		metadata["n"] = *request.Output.Count
	}
	if request.Output.Size != nil {
		legacy.Size = *request.Output.Size
	}
	if request.Output.AspectRatio != nil {
		legacy.AspectRatio = request.Output.AspectRatio
		metadata["aspect_ratio"] = *request.Output.AspectRatio
	}
	if request.Output.Resolution != nil {
		legacy.Resolution = request.Output.Resolution
		metadata["resolution"] = *request.Output.Resolution
	}
	legacy.Quality = request.Output.Quality
	if request.Output.Quality != nil {
		metadata["quality"] = *request.Output.Quality
	}
	if request.Output.Format != nil {
		metadata["output_format"] = *request.Output.Format
	}
	if request.Output.Compression != nil {
		metadata["output_compression"] = *request.Output.Compression
	}
	if request.Output.Background != nil {
		metadata["background"] = *request.Output.Background
	}
	return legacy
}
