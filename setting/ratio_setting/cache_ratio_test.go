package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultCreateCacheRatioIncludesGPT56Models(t *testing.T) {
	for _, model := range []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"} {
		require.Equal(t, 1.25, defaultCreateCacheRatio[model], model)
	}
}

func TestGPT56AliasFallsBackToConfiguredSolCacheRatios(t *testing.T) {
	originalCache := CacheRatio2JSONString()
	originalCreate := CreateCacheRatio2JSONString()
	t.Cleanup(func() {
		if err := UpdateCacheRatioByJSONString(originalCache); err != nil {
			t.Fatal(err)
		}
		if err := UpdateCreateCacheRatioByJSONString(originalCreate); err != nil {
			t.Fatal(err)
		}
	})
	if err := UpdateCacheRatioByJSONString(`{"gpt-5.6-sol":0.2}`); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCreateCacheRatioByJSONString(`{"gpt-5.6-sol":1.5}`); err != nil {
		t.Fatal(err)
	}
	if ratio, ok := GetCacheRatio("gpt-5.6"); !ok || ratio != 0.2 {
		t.Fatalf("expected Sol cache ratio fallback, got %v, %v", ratio, ok)
	}
	if ratio, ok := GetCreateCacheRatio("gpt-5.6"); !ok || ratio != 1.5 {
		t.Fatalf("expected Sol cache write ratio fallback, got %v, %v", ratio, ok)
	}
}
