package main

import "testing"

func TestShouldOpenAlert(t *testing.T) {
	cases := []struct {
		name         string
		breached     bool
		hasOpenAlert bool
		want         bool
	}{
		{"new problem, nothing open yet", true, false, true},
		{"problem continues, already open", true, true, false},
		{"no problem, nothing open", false, false, false},
		{"no problem, but something is open", false, true, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldOpenAlert(c.breached, c.hasOpenAlert)
			if got != c.want {
				t.Errorf("shouldOpenAlert(%v, %v) = %v, want %v", c.breached, c.hasOpenAlert, got, c.want)
			}
		})
	}
}

func TestShouldResolveAlert(t *testing.T) {
	cases := []struct {
		name         string
		breached     bool
		hasOpenAlert bool
		want         bool
	}{
		{"problem cleared, alert was open", false, true, true},
		{"problem cleared, nothing was open", false, false, false},
		{"problem still there, alert open", true, true, false},
		{"problem still there, nothing open", true, false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldResolveAlert(c.breached, c.hasOpenAlert)
			if got != c.want {
				t.Errorf("shouldResolveAlert(%v, %v) = %v, want %v", c.breached, c.hasOpenAlert, got, c.want)
			}
		})
	}
}

func TestIsRestartSpike(t *testing.T) {
	cases := []struct {
		restarts int32
		want     bool
	}{
		{0, false},
		{3, false}, // boundary: exactly at threshold, not over
		{4, true},
		{10, true},
	}

	for _, c := range cases {
		got := isRestartSpike(c.restarts)
		if got != c.want {
			t.Errorf("isRestartSpike(%d) = %v, want %v", c.restarts, got, c.want)
		}
	}
}

func TestIsMemoryOverThreshold(t *testing.T) {
	const mi = 1024 * 1024
	cases := []struct {
		memoryBytes int64
		want        bool
	}{
		{100 * mi, false},
		{200 * mi, false}, // boundary
		{201 * mi, true},
		{300 * mi, true},
	}

	for _, c := range cases {
		got := isMemoryOverThreshold(c.memoryBytes)
		if got != c.want {
			t.Errorf("isMemoryOverThreshold(%d) = %v, want %v", c.memoryBytes, got, c.want)
		}
	}
}
