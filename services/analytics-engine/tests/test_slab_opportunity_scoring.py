import sys
import unittest
from pathlib import Path
from decimal import Decimal

SYS_PATH = Path(__file__).resolve().parents[1]
if str(SYS_PATH) not in sys.path:
    sys.path.insert(0, str(SYS_PATH))

from slab_opportunity_scorer import score_listing
from slab_valuation import value_from_comps


class SlabOpportunityScoringTest(unittest.TestCase):
    def test_values_from_sold_comps_with_outlier_filtering(self):
        valuation = value_from_comps([
            {"sold_price": "100"},
            {"sold_price": "110"},
            {"sold_price": "120"},
            {"sold_price": "1000"},
        ])

        self.assertEqual(valuation.market_value, Decimal("110.00"))
        self.assertEqual(valuation.comp_count, 3)
        self.assertGreater(valuation.confidence_score, 0)

    def test_scores_candidate_when_margin_and_confidence_are_strong(self):
        listing = {
            "external_card_id": "smp-SM228",
            "card_name": "Armored Mewtwo",
            "set_name": "SM Promos",
            "is_slab": True,
            "grader": "PSA",
            "grade": "10",
            "slab_tier": "PSA_10",
            "marketplace": "ebay",
            "listing_id": "listing-1",
            "listing_url": "https://example.com/listing-1",
            "listing_title": "Armored Mewtwo SM228 PSA 10",
            "price": "200",
        }
        comps = [
            {"sold_price": "360"},
            {"sold_price": "375"},
            {"sold_price": "390"},
            {"sold_price": "410"},
            {"sold_price": "380"},
        ]

        result = score_listing(listing, comps)

        self.assertIsNotNone(result)
        self.assertEqual(result.decision, "candidate")
        self.assertGreater(result.expected_profit, Decimal("100"))
        self.assertGreater(result.deal_score, 80)

    def test_recent_comps_raise_trend_score_and_deal_score(self):
        rising = value_from_comps([
            {"sold_price": "100", "sold_at": "2026-03-01T00:00:00+00:00"},
            {"sold_price": "105", "sold_at": "2026-03-10T00:00:00+00:00"},
            {"sold_price": "150", "sold_at": "2026-05-20T00:00:00+00:00"},
            {"sold_price": "160", "sold_at": "2026-05-25T00:00:00+00:00"},
        ], now="2026-05-31T00:00:00+00:00")
        flat = value_from_comps([
            {"sold_price": "120", "sold_at": "2026-03-01T00:00:00+00:00"},
            {"sold_price": "121", "sold_at": "2026-03-10T00:00:00+00:00"},
            {"sold_price": "122", "sold_at": "2026-05-20T00:00:00+00:00"},
            {"sold_price": "123", "sold_at": "2026-05-25T00:00:00+00:00"},
        ], now="2026-05-31T00:00:00+00:00")

        self.assertGreater(rising.trend_score, flat.trend_score)
        self.assertIn("recent", rising.reason.lower())

    def test_rejects_raw_listing(self):
        result = score_listing({"card_name": "Charizard", "price": "10", "is_slab": False}, [])
        self.assertIsNone(result)

    def test_missing_comps_does_not_invent_market_value(self):
        listing = {
            "card_name": "Charizard",
            "is_slab": True,
            "slab_tier": "PSA_10",
            "price": "50",
        }

        result = score_listing(listing, [])

        self.assertIsNotNone(result)
        self.assertEqual(result.estimated_market_value, Decimal("0.00"))
        self.assertEqual(result.decision, "watch")


if __name__ == "__main__":
    unittest.main()
