package app

import (
	"testing"

	"nte-optimizer/internal/scoring"
	"nte-optimizer/internal/target"
)

func TestBuildRecommendationProfilesUsesStaticVariants(t *testing.T) {
	recommendation := target.BuildTarget{
		ID:   "shinku",
		Name: "Shinku",
		Character: target.CharacterConfig{
			CharacterID: 1076,
			GridID:      "grid-shinku",
		},
		Builds: []target.BuildProfile{{
			ID:        "shinku",
			Name:      "DPS",
			MainStats: []string{"CritBase"},
			Weights:   map[string]float64{"CritBase": .85},
			Variants: []target.ProfileVariant{{
				ID:   "shinku_lost_radiance",
				Name: "Shinku — DPS — Lost Radiance",
				PreferredSets: []scoring.SetPreference{{
					SetID: "Suit6", Priority: 1,
				}},
			}},
		}},
	}

	profiles := BuildRecommendationProfiles(recommendation)
	profile, ok := profiles["shinku_lost_radiance"]
	if !ok {
		t.Fatalf("expected static variant, got %#v", profiles)
	}
	if profile.CharacterID != 1076 || profile.GridID != "grid-shinku" || profile.TargetID != "shinku" {
		t.Fatalf("recommendation context was not preserved: %#v", profile)
	}
	if profile.Name != "Shinku — DPS — Lost Radiance" || len(profile.PreferredSets) != 1 || profile.PreferredSets[0].SetID != "Suit6" {
		t.Fatalf("unexpected profile variant: %#v", profile)
	}
	if len(profile.MainStats) != 1 || profile.MainStats[0] != "CritBase" || profile.Weights["CritBase"] != .85 {
		t.Fatalf("unexpected recommendation weights: %#v", profile)
	}
}

func TestBuildRecommendationProfilesSupportsAlternateCharacterIDs(t *testing.T) {
	recommendation := target.BuildTarget{
		ID: "zero", Name: "Zero",
		Character: target.CharacterConfig{CharacterID: 1046, AlternateIDs: []int{1051}, GridID: "character_1046"},
		Builds:    []target.BuildProfile{{ID: "zero", Weights: map[string]float64{"CritBase": .8}, Variants: []target.ProfileVariant{{ID: "zero", Name: "Zero — Speedy Hedgehog"}}}},
	}
	profiles := BuildRecommendationProfiles(recommendation)
	if profiles["zero"].CharacterID != 1046 || profiles["zero"].GridID != "character_1046" {
		t.Fatalf("primary Zero profile changed: %+v", profiles["zero"])
	}
	alternate := profiles["zero_1051"]
	if alternate.CharacterID != 1051 || alternate.GridID != "character_1051" || alternate.TargetID != "zero" || alternate.Weights["CritBase"] != .8 {
		t.Fatalf("alternate Zero profile is incomplete: %+v", alternate)
	}
}
