package ratio_setting

import "testing"

func TestGetCompletionRatioInfoForGPT55IsLockedToSix(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5.5")
	if !info.Locked {
		t.Fatalf("expected gpt-5.5 completion ratio to be locked")
	}
	if info.Ratio != 6 {
		t.Fatalf("expected gpt-5.5 completion ratio 6, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioInfoForGPT54RemainsLockedToSix(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5.4")
	if !info.Locked {
		t.Fatalf("expected gpt-5.4 completion ratio to be locked")
	}
	if info.Ratio != 6 {
		t.Fatalf("expected gpt-5.4 completion ratio 6, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioInfoForGPT54NanoRemainsLockedToSixPointTwoFive(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5.4-nano")
	if !info.Locked {
		t.Fatalf("expected gpt-5.4-nano completion ratio to be locked")
	}
	if info.Ratio != 6.25 {
		t.Fatalf("expected gpt-5.4-nano completion ratio 6.25, got %v", info.Ratio)
	}
}

func TestGetCompletionRatioInfoForGPT5RemainsLockedToEight(t *testing.T) {
	info := GetCompletionRatioInfo("gpt-5")
	if !info.Locked {
		t.Fatalf("expected gpt-5 completion ratio to be locked")
	}
	if info.Ratio != 8 {
		t.Fatalf("expected gpt-5 completion ratio 8, got %v", info.Ratio)
	}
}
