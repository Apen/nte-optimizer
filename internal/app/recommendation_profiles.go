package app

import (
	"nte-optimizer/internal/scoring"
	"nte-optimizer/internal/target"
)

// BuildRecommendationProfiles converts the static recommendation catalog into
// the scoring profiles exposed by the optimizer.
func BuildRecommendationProfiles(recommendation target.BuildTarget) map[string]scoring.Character {
	profiles := make(map[string]scoring.Character)
	for _, build := range recommendation.Builds {
		base := recommendationTemplate(recommendation)
		base.Name = recommendation.Name
		if build.Name != "" {
			base.Name += " — " + build.Name
		}
		base.MainStats = append([]string(nil), build.MainStats...)
		base.Weights = cloneWeights(build.Weights)
		base.MainWeights = nil
		base.SubWeights = nil
		base.Caps = nil
		base.PreferredSets = nil

		if len(build.Variants) == 0 {
			profiles[build.ID] = base
			continue
		}
		for _, variant := range build.Variants {
			profile := base
			profile.Name = variant.Name
			profile.PreferredSets = append([]scoring.SetPreference(nil), variant.PreferredSets...)
			profiles[variant.ID] = profile
		}
	}
	return profiles
}

func recommendationTemplate(recommendation target.BuildTarget) scoring.Character {
	config := recommendation.Character
	return scoring.Character{
		CharacterID:  config.CharacterID,
		Name:         recommendation.Name,
		TargetID:     recommendation.ID,
		GridID:       config.GridID,
		CurrentStats: cloneWeights(config.CurrentStats),
		BaseStats:    cloneWeights(config.BaseStats),
		ConsoleTrait: config.ConsoleTrait,
		Caps:         config.Caps,
	}
}
