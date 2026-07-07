package ratio_setting

import "testing"

func TestGetCompletionRatioInfoForGPT55IsConfigurableWithDefaultSix(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5.5")
	if info.Locked {
		t.Fatalf("expected gpt-5.5 completion ratio to be configurable")
	}
	if info.Ratio != 6 {
		t.Fatalf("expected gpt-5.5 completion ratio 6, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioInfoForGPT54IsConfigurableWithDefaultSix(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5.4")
	if info.Locked {
		t.Fatalf("expected gpt-5.4 completion ratio to be configurable")
	}
	if info.Ratio != 6 {
		t.Fatalf("expected gpt-5.4 completion ratio 6, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioInfoForGPT54NanoIsConfigurableWithDefaultSixPointTwoFive(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5.4-nano")
	if info.Locked {
		t.Fatalf("expected gpt-5.4-nano completion ratio to be configurable")
	}
	if info.Ratio != 6.25 {
		t.Fatalf("expected gpt-5.4-nano completion ratio 6.25, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioInfoForGPT5IsConfigurableWithDefaultEight(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5")
	if info.Locked {
		t.Fatalf("expected gpt-5 completion ratio to be configurable")
	}
	if info.Ratio != 8 {
		t.Fatalf("expected gpt-5 completion ratio 8, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioForGPT5UsesConfiguredRatio(t *testing.T) {
	original := CompletionRatio2JSONString()
	t.Cleanup(func() {
		if err := UpdateCompletionRatioByJSONString(original); err != nil {
			t.Fatalf("failed to restore completion ratio map: %v", err)
		}
	})

	if err := UpdateCompletionRatioByJSONString(`{"gpt-5":3.5,"gpt-5.6-luna":4.5}`); err != nil {
		t.Fatalf("failed to update completion ratio map: %v", err)
	}

	if ratio := GetCompletionRatio("gpt-5"); ratio != 3.5 {
		t.Fatalf("expected configured gpt-5 completion ratio 3.5, got %v", ratio)
	}

	customInfo := GetCompletionRatioInfo("gpt-5.6-luna")
	if customInfo.Locked {
		t.Fatalf("expected custom gpt-5 variant completion ratio to be configurable")
	}
	if customInfo.Ratio != 4.5 {
		t.Fatalf("expected configured gpt-5.6-luna completion ratio 4.5, got %v", customInfo.Ratio)
	}
}
