package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalNoEscapeHTMLPreservesURLQuerySeparators(t *testing.T) {
	t.Parallel()

	value := map[string]string{
		"url": "https://example.com/image.png?first=1&second=2",
	}

	encoded, err := MarshalNoEscapeHTML(value)
	require.NoError(t, err)
	require.JSONEq(t, `{"url":"https://example.com/image.png?first=1&second=2"}`, string(encoded))
	require.Contains(t, string(encoded), "first=1&second=2")
	require.NotContains(t, string(encoded), `\u0026`)
	require.False(t, len(encoded) > 0 && encoded[len(encoded)-1] == '\n')
}
