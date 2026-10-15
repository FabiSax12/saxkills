package version

import "testing"

func TestNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.0", "v1.1.9", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.2.0", "v1.3.0", false},
		{"v2.0.0", "v1.99.99", true},
		{"v1.10.0", "v1.9.0", true},
		{"1.2.0", "v1.1.0", true},
		{"v1.2.0", "dev", false},
		{"v1.2.0", "", false},
		{"nightly", "v1.0.0", false},
		{"v1.2.0-rc1", "v1.1.0", true},
	}
	for _, tt := range tests {
		if got := Newer(tt.latest, tt.current); got != tt.want {
			t.Errorf("Newer(%q,%q)=%v want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestDiffers(t *testing.T) {
	tests := []struct {
		latest, installed string
		want              bool
	}{
		{"v1.2.0", "v1.1.0", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.0.0", "v1.2.0", false},
		{"nightly", "v1", true},
		{"v1.0.0", "", true},
	}
	for _, tt := range tests {
		if got := Outdated(tt.installed, tt.latest); got != tt.want {
			t.Errorf("Outdated(%q,%q)=%v want %v", tt.installed, tt.latest, got, tt.want)
		}
	}
}
