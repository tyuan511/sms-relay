package telegram

import "testing"

func TestMatchStartPayload(t *testing.T) {
	secret := "abc123"
	tests := []struct {
		text string
		want bool
	}{
		{"/start abc123", true},
		{"/start link_abc123", true},
		{"/start abc123@mybot", true},
		{"/start link_wrong", false},
		{"/start", false},
		{"hello", false},
	}
	for _, tc := range tests {
		if got := MatchStartPayload(tc.text, secret); got != tc.want {
			t.Fatalf("MatchStartPayload(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestShouldBindMessage(t *testing.T) {
	secret := "abc123"
	if ShouldBindMessage("/start", secret, "private") {
		t.Fatal("plain /start should not bind")
	}
	if ShouldBindMessage("/start", secret, "group") {
		t.Fatal("plain /start in group should not bind")
	}
	if !ShouldBindMessage("/start abc123", secret, "group") {
		t.Fatal("deep link /start should bind in group")
	}
}
