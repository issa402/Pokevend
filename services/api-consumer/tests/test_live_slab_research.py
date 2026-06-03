import json
import tempfile
from unittest import TestCase, main

from services.live_slab_research import (
    CardTarget,
    load_targets_config,
    is_credible_slab_listing,
    score_listing_candidate,
    classify_signal,
    target_buy_price,
    research_target_candidate,
    target_from_mover,
)


class LiveSlabResearchTest(TestCase):

    def test_load_targets_config_uses_curated_identity_terms(self):
        tmp = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False)
        with tmp:
            json.dump({
                "targets": [{
                    "title": "Armored Mewtwo #SM228",
                    "setName": "Pokemon Promo",
                    "priceChartingUrl": "https://example.com/armored",
                    "referenceValue": 8000,
                    "requiredTerms": ["armored", "mewtwo"],
                    "identityTerms": ["sm228"]
                }]
            }, tmp)

        targets = load_targets_config(tmp.name)

        self.assertEqual(len(targets), 1)
        self.assertEqual(targets[0].identity_terms, ("sm228",))
        self.assertEqual(targets[0].reference_value, 8000.0)

    def test_rejects_wrong_charizard_variant_false_positive(self):
        target = CardTarget(
            title="Charizard #146",
            set_name="Pokemon Skyridge",
            url="https://example.com/skyridge-charizard",
            required_terms=("charizard",),
            identity_terms=("146", "skyridge"),
            target_slab_tier="PSA_10",
            reference_value=76388.42,
            trend_change=725.54,
        )

        self.assertFalse(is_credible_slab_listing(
            "Pokemon Japanese VMax Climax Charizard 187/184 Character Secret Rare PSA 10",
            target,
        ))


    def test_rejects_celebrations_reprint_for_pop_series_gold_star(self):
        target = CardTarget(
            title="Umbreon [Gold Star] #17",
            set_name="Pokemon POP Series 5",
            url="https://example.com/umbreon-gold-star",
            required_terms=("umbreon", "gold", "star"),
            identity_terms=("17", "series"),
            target_slab_tier="PSA_10",
            reference_value=63460.5,
            trend_change=475.34,
        )

        self.assertFalse(is_credible_slab_listing(
            "Pokemon Umbreon Gold Star Celebrations Classic Coll POP Series 5 Holo #17 PSA 10",
            target,
        ))

    def test_accepts_exact_card_number_and_grade(self):
        target = CardTarget(
            title="Armored Mewtwo #SM228",
            set_name="Pokemon Promo",
            url="https://example.com/armored",
            required_terms=("armored", "mewtwo"),
            identity_terms=("sm228",),
            target_slab_tier="PSA_10",
            reference_value=8000.0,
            trend_change=100.0,
        )

        self.assertTrue(is_credible_slab_listing(
            "2019 Pokemon SM Black Star Promo SM228 Armored Mewtwo PSA 10 GEM MINT",
            target,
        ))

    def test_classifies_above_reference_ask_as_sell_research(self):
        self.assertEqual(classify_signal(expected_profit=-100, expected_margin_pct=-10, min_profit=25, min_margin_pct=20), "SELL_RESEARCH")
        self.assertEqual(classify_signal(expected_profit=100, expected_margin_pct=25, min_profit=25, min_margin_pct=20), "BUY_CANDIDATE")
        self.assertEqual(classify_signal(expected_profit=10, expected_margin_pct=25, min_profit=25, min_margin_pct=20), "SELL_RESEARCH")

    def test_scores_candidate_profit_after_fees(self):
        result = score_listing_candidate(reference_value=8000, asking_price=5000, fee_rate=0.14)

        self.assertEqual(result["allInCost"], 5700.0)
        self.assertEqual(result["expectedProfit"], 2300.0)
        self.assertGreater(result["expectedMarginPct"], 40)


if __name__ == "__main__":
    main()


class TargetBuyPriceTest(TestCase):
    def test_target_buy_price_accounts_for_fees_profit_and_margin(self):
        price = target_buy_price(reference_value=8000, min_profit=25, min_margin_pct=20, fee_rate=0.14)

        self.assertEqual(price, 5847.95)



class ResearchTargetCandidateTest(TestCase):
    def test_builds_pricecharting_research_target_with_ebay_search_url(self):
        target = CardTarget(
            title="Charizard #146",
            set_name="Pokemon Skyridge",
            url="https://www.pricecharting.com/game/pokemon-skyridge/charizard-146",
            required_terms=("charizard",),
            identity_terms=("146", "skyridge"),
            target_slab_tier="PSA_10",
            reference_value=76000,
            trend_change=725,
        )

        candidate = research_target_candidate(target, min_profit=25, min_margin_pct=20, fee_rate=0.14)

        self.assertEqual(candidate.signal, "RESEARCH_TARGET")
        self.assertEqual(candidate.marketplace, "pricecharting")
        self.assertEqual(candidate.listing_id, "pricecharting:charizard 146:pokemon skyridge:PSA_10")
        self.assertIn("ebay.com/sch/i.html", candidate.listing_url)
        self.assertEqual(candidate.target_buy_price, 55555.56)


class PriceChartingMoverParsingTest(TestCase):
    def test_target_from_mover_strips_pricecharting_html_whitespace(self):
        from unittest.mock import patch
        from services.pricecharting_market import PriceChartingMover

        mover = PriceChartingMover(
            title="\\n \n                        Charizard #146\\n \n",
            set_name="\\n                    Pokemon Skyridge\\n",
            url="https://www.pricecharting.com/game/pokemon-skyridge/charizard-146",
            loose_price=1200.0,
            change_amount=425.75,
        )

        with patch("services.live_slab_research.fetch_grade_prices", return_value={"PSA_10": 79152.19}):
            target = target_from_mover(mover)

        self.assertIsNotNone(target)
        self.assertEqual(target.title, "Charizard #146")
        self.assertEqual(target.set_name, "Pokemon Skyridge")
        self.assertEqual(target.identity_terms, ("146", "skyridge"))
