from unittest import TestCase, main

from services.live_slab_research import LiveSlabCandidate
from repositories.live_slab_opportunity_repo import to_opportunity_record


class LiveSlabOpportunityRepoTest(TestCase):
    def test_maps_buy_candidate_to_candidate_decision(self):
        candidate = LiveSlabCandidate(
            card_title="Armored Mewtwo #SM228",
            set_name="Pokemon Promo",
            target_slab_tier="PSA_10",
            reference_value=8000,
            target_buy_price=5847.95,
            asking_price=5000,
            all_in_cost=5700,
            expected_profit=2300,
            expected_margin_pct=40.35,
            signal="BUY_CANDIDATE",
            trend_change=100,
            listing_title="2019 Pokemon SM228 Armored Mewtwo PSA 10",
            listing_url="https://www.ebay.com/itm/123456789012",
            listing_id="123456789012",
            reference_url="https://www.pricecharting.com/game/pokemon-promo/armored-mewtwo-sm228",
        )

        record = to_opportunity_record(candidate)

        self.assertEqual(record["decision"], "candidate")
        self.assertEqual(record["card_name"], "Armored Mewtwo #SM228")
        self.assertEqual(record["marketplace"], "ebay")
        self.assertEqual(record["listing_id"], "123456789012")
        self.assertEqual(record["deal_score"], 246)
        self.assertIn("BUY_CANDIDATE", record["reason"])
        self.assertEqual(record["evidence"]["signal"], "BUY_CANDIDATE")
        self.assertEqual(record["evidence"]["targetBuyPrice"], 5847.95)

    def test_maps_sell_research_to_watch_decision(self):
        candidate = LiveSlabCandidate(
            card_title="Armored Mewtwo #SM228",
            set_name="Pokemon Promo",
            target_slab_tier="PSA_10",
            reference_value=8000,
            target_buy_price=5847.95,
            asking_price=10875.05,
            all_in_cost=12397.56,
            expected_profit=-4397.56,
            expected_margin_pct=-35.47,
            signal="SELL_RESEARCH",
            trend_change=None,
            listing_title="PSA 10 Armored Mewtwo - SM228",
            listing_url="https://www.ebay.com/itm/287259624468",
            listing_id="287259624468",
            reference_url="https://www.pricecharting.com/game/pokemon-promo/armored-mewtwo-sm228",
        )

        record = to_opportunity_record(candidate)

        self.assertEqual(record["decision"], "watch")
        self.assertEqual(record["deal_score"], 0)
        self.assertEqual(record["evidence"]["signal"], "SELL_RESEARCH")


if __name__ == "__main__":
    main()


class LiveSlabResearchTargetMappingTest(TestCase):
    def test_maps_pricecharting_research_target_to_watch_row(self):
        candidate = LiveSlabCandidate(
            card_title="Charizard #146",
            set_name="Pokemon Skyridge",
            target_slab_tier="PSA_10",
            reference_value=76000,
            target_buy_price=55555.56,
            asking_price=0,
            all_in_cost=0,
            expected_profit=0,
            expected_margin_pct=0,
            signal="RESEARCH_TARGET",
            trend_change=725,
            listing_title="Research target from PriceCharting: Charizard #146 PSA_10",
            listing_url="https://www.ebay.com/sch/i.html?_nkw=Charizard",
            listing_id="pricecharting:charizard 146:pokemon skyridge:PSA_10",
            reference_url="https://www.pricecharting.com/game/pokemon-skyridge/charizard-146",
            marketplace="pricecharting",
        )

        record = to_opportunity_record(candidate)

        self.assertEqual(record["decision"], "watch")
        self.assertEqual(record["marketplace"], "pricecharting")
        self.assertEqual(record["listing_id"], "pricecharting:charizard 146:pokemon skyridge:PSA_10")
        self.assertEqual(record["evidence"]["signal"], "RESEARCH_TARGET")
        self.assertTrue(record["evidence"]["needsEbayScan"])
        self.assertIn("target buy <= $55555.56", record["reason"])
        self.assertGreater(record["deal_score"], 0)
