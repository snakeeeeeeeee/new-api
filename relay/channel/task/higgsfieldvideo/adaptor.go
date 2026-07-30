package higgsfieldvideo

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/adobevideo"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	ChannelName              = "higgsfield-video"
	ProviderOptionsNamespace = "higgsfield_video"
)

var ModelList = []string{
	"seedance-2.0-480p",
	"seedance-2.0-720p",
}

var supportedModels = map[string]struct{}{
	"seedance-2.0-480p": {},
	"seedance-2.0-720p": {},
}

type TaskAdaptor struct {
	adobevideo.TaskAdaptor
}

var (
	_ channel.TaskAdaptor                = (*TaskAdaptor)(nil)
	_ channel.NormalizedVideoTaskAdaptor = (*TaskAdaptor)(nil)
	_ channel.VideoBillingEstimator      = (*TaskAdaptor)(nil)
	_ channel.VideoContentResolver       = (*TaskAdaptor)(nil)
)

func (a *TaskAdaptor) PrepareNormalizedVideoRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	request dto.VideoTaskCreateRequest,
) *dto.TaskError {
	options, hasHiggsfieldOptions := request.ProviderOptions[ProviderOptionsNamespace]
	_, hasAdobeOptions := request.ProviderOptions[adobevideo.ProviderOptionsNamespace]
	if hasAdobeOptions {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("provider_options.%s is not supported by HiggsfieldVideo", adobevideo.ProviderOptionsNamespace),
			"invalid_provider_options",
			http.StatusBadRequest,
		)
	}
	if hasHiggsfieldOptions {
		request.ProviderOptions = cloneProviderOptions(request.ProviderOptions)
		delete(request.ProviderOptions, ProviderOptionsNamespace)
		request.ProviderOptions[adobevideo.ProviderOptionsNamespace] = options
	}
	return a.TaskAdaptor.PrepareNormalizedVideoRequest(c, info, request)
}

func (a *TaskAdaptor) GetModelList() []string {
	return append([]string(nil), ModelList...)
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ValidateNormalizedVideoModel(
	_ *gin.Context,
	info *relaycommon.RelayInfo,
) *dto.TaskError {
	if info == nil || info.ChannelMeta == nil {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("relay info is required"),
			"invalid_request",
			http.StatusBadRequest,
		)
	}
	model := strings.TrimSpace(info.UpstreamModelName)
	if _, ok := supportedModels[model]; !ok {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("unsupported HiggsfieldVideo provider SKU %q", model),
			"unsupported_video_model",
			http.StatusBadRequest,
		)
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(
	c *gin.Context,
	info *relaycommon.RelayInfo,
) (io.Reader, error) {
	return a.TaskAdaptor.BuildRequestBodyForProvider(
		c,
		info,
		"HiggsfieldVideo",
		supportedModels,
	)
}

func cloneProviderOptions(source map[string]map[string]any) map[string]map[string]any {
	cloned := make(map[string]map[string]any, len(source))
	for namespace, options := range source {
		cloned[namespace] = options
	}
	return cloned
}
