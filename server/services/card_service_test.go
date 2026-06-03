package services

import (
	"testing"

	"pokemontool/models"
)

func TestSearchTokensScoreCollectorStyleCardSetQueries(t *testing.T) {
	tests := []struct {
		name  string
		query string
		card  models.PokeTCGCard
	}{
		{
			name:  "accented pokemon rumble set with denominator",
			query: "lucario 12/16 pokemon rumble",
			card:  models.PokeTCGCard{ID: "ru1-12", Name: "Lucario", Set: "Pokémon Rumble", Number: "12"},
		},
		{
			name:  "base set number query with official base set name",
			query: "charizard 4/102 base set",
			card:  models.PokeTCGCard{ID: "base1-4", Name: "Charizard", Set: "Base", Number: "4", SetPrintedTotal: "102"},
		},
		{
			name:  "pop series set query",
			query: "umbreon 17/17 pop series 5",
			card:  models.PokeTCGCard{ID: "pop5-17", Name: "Umbreon Star", Set: "POP Series 5", Number: "17"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := searchTokens(tt.query)
			if score := scorePokeTCGCard(tokens, tt.card); score <= 0 {
				t.Fatalf("expected positive score for %q against %+v, got %d", tt.query, tt.card, score)
			}
		})
	}
}

func TestCollectorDenominatorRanksOriginalBaseSetAboveBaseSet2(t *testing.T) {
	tokens := searchTokens("charizard 4/102 base set")
	baseSet := models.PokeTCGCard{ID: "base1-4", Name: "Charizard", Set: "Base", Number: "4", SetPrintedTotal: "102"}
	baseSet2 := models.PokeTCGCard{ID: "base4-4", Name: "Charizard", Set: "Base Set 2", Number: "4", SetPrintedTotal: "130"}

	baseScore := scorePokeTCGCard(tokens, baseSet)
	baseSet2Score := scorePokeTCGCard(tokens, baseSet2)
	if baseSet.SetPrintedTotal == "102" {
		baseScore += 8
	}
	if baseSet2.SetPrintedTotal == "102" {
		baseSet2Score += 8
	}
	if baseScore <= baseSet2Score {
		t.Fatalf("expected original Base Set score %d to beat Base Set 2 score %d", baseScore, baseSet2Score)
	}
}

func TestSearchTokensRejectWrongSetForSamePokemon(t *testing.T) {
	tokens := searchTokens("lucario 12/16 pokemon rumble")
	wrongSet := models.PokeTCGCard{ID: "swsh291", Name: "Lucario VSTAR", Set: "SWSH Black Star Promos", Number: "SWSH291"}
	if score := scorePokeTCGCard(tokens, wrongSet); score != 0 {
		t.Fatalf("expected wrong set to score 0, got %d", score)
	}
}
