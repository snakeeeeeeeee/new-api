package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeImageHandleExecutionDriver(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		value    string
		expected string
	}{
		{name: "missing", value: "", expected: ImageHandleExecutionDriverLegacySync},
		{name: "legacy", value: " legacy_sync ", expected: ImageHandleExecutionDriverLegacySync},
		{name: "adobe async", value: "ADOBE2API_ASYNC_IMAGE_V1", expected: ImageHandleExecutionDriverAdobeAsyncImage},
		{name: "unknown", value: "future_driver", expected: ImageHandleExecutionDriverLegacySync},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, NormalizeImageHandleExecutionDriver(testCase.value))
		})
	}
}
