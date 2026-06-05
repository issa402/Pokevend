from unittest import TestCase, main
from pathlib import Path

from services.seller_hub_research import _default_profile_dir, build_keywords, parse_research_text, research_url


SELLER_HUB_TEXT = """
Research products
Sold
Active
Currently live today
Show current trends
$2,024.71
Avg listing price
$0.99 - $24,999.99
Listing price range
$35.22
Avg shipping
21%
Free shipping
233
Total active listings
31%
Promoted listings
Listing
Actions
Listing price
Bids
Watchers
Promoted listing
Start date
2019 POKEMON SUN & MOON TEAM UP SECRET #186 FULL ART/GENGAR & MIMIKYU GX PSA 10
, preview full size image
2019 POKEMON SUN & MOON TEAM UP SECRET #186 FULL ART/GENGAR & MIMIKYU GX PSA 10
$2,199.99
Free shipping
-
5
Jun 1, 2026
Pokemon Gengar & Mimikyu GX Team Up Full Alt Art #165 PSA 10 Gem Mint
, preview full size image
Pokemon Gengar & Mimikyu GX Team Up Full Alt Art #165 PSA 10 Gem Mint
$4,650.00
+$9.99 shipping
9
81
May 31, 2026
"""


class SellerHubResearchTest(TestCase):
    def test_default_profile_dir_supports_shallow_container_path(self):
        self.assertEqual(
            _default_profile_dir(Path("/app/services/seller_hub_research.py")),
            Path("/app/.local/ebay-seller-hub-profile"),
        )

    def test_build_keywords_includes_number_set_and_grade(self):
        self.assertEqual(
            build_keywords("Gengar & Mimikyu GX", "Team Up", "165", "PSA_10"),
            "Gengar & Mimikyu GX 165 Team Up PSA 10",
        )


    def test_build_keywords_does_not_duplicate_number_from_card_name(self):
        self.assertEqual(
            build_keywords("Charizard [1st Edition] #4", "Base", "4", "PSA_10"),
            "Charizard [1st Edition] #4 Base PSA 10",
        )

    def test_research_url_targets_seller_hub(self):
        url = research_url("Pikachu PSA 10", tab_name="ACTIVE", day_range=30)
        self.assertIn("https://www.ebay.com/sh/research?", url)
        self.assertIn("keywords=Pikachu+PSA+10", url)
        self.assertIn("tabName=ACTIVE", url)

    def test_parse_research_text_extracts_metrics_and_rows(self):
        metrics = parse_research_text(
            SELLER_HUB_TEXT,
            external_card_id="sm9-165",
            card_name="Gengar & Mimikyu GX",
            set_name="Team Up",
            card_number="165",
            language_preference="ENGLISH",
            slab_tier="PSA_10",
            tab_name="ACTIVE",
            keywords="Gengar & Mimikyu GX 165 Team Up PSA 10",
            day_range=30,
            source_url="https://www.ebay.com/sh/research?...",
        )

        self.assertEqual(metrics.avg_listing_price, 4650.0)
        self.assertEqual(metrics.min_listing_price, 4650.0)
        self.assertEqual(metrics.max_listing_price, 4650.0)
        self.assertEqual(metrics.avg_shipping_price, 9.99)
        self.assertEqual(metrics.free_shipping_pct, 0.0)
        self.assertEqual(metrics.promoted_listing_pct, 0.0)
        self.assertEqual(metrics.total_listings, 1)
        self.assertEqual(len(metrics.top_rows), 1)
        self.assertNotIn("#186", metrics.top_rows[0].title)
        self.assertIn("#165", metrics.top_rows[0].title)
        self.assertEqual(metrics.top_rows[0].shipping, 9.99)
        self.assertEqual(metrics.top_rows[0].bids, 9)
        self.assertEqual(metrics.top_rows[0].watchers, 81)
        self.assertEqual(metrics.top_rows[0].slab_tier, "PSA_10")

    def test_parse_research_text_rejects_conflicting_fraction(self):
        text = """
Research products
$100.00
Avg listing price
Listing
Actions
Listing price
Bids
Watchers
Start date
Spinda 25/92 EX Legend Maker PSA 7
$50.00
Free shipping
-
1
Jun 1, 2026
Spinda 26/92 EX Legend Maker PSA 7
$125.00
+$5.99 shipping
-
3
Jun 2, 2026
"""
        metrics = parse_research_text(
            text,
            external_card_id="ex12-26",
            card_name="Spinda",
            set_name="Legend Maker",
            card_number="26/92",
            language_preference="ENGLISH",
            slab_tier="PSA_7",
            tab_name="ACTIVE",
            keywords="Spinda 26/92 Legend Maker PSA 7",
            day_range=30,
        )

        self.assertEqual(metrics.total_listings, 1)
        self.assertEqual(metrics.avg_listing_price, 125.0)
        self.assertEqual(metrics.top_rows[0].title, "Spinda 26/92 EX Legend Maker PSA 7")


    def test_parse_research_text_infers_card_number_from_card_name(self):
        text = """
Research products
Listing
Actions
Listing price
Bids
Watchers
Start date
2009 POKEMON JAPANESE ADVENT OF ARCEUS 1ST EDITION #017 CHARIZARD-HOLO PSA 10
$1,600.00
+$5.99 shipping
-
22
May 20, 2026
1999 Pokemon Base Set 1st Edition Shadowless Holo Charizard #4 GEM MINT - PSA 10
$750,000.00
+$116.80 shipping
-
257
Feb 23, 2026
"""
        metrics = parse_research_text(
            text,
            external_card_id=None,
            card_name="Charizard [1st Edition] #4",
            set_name=None,
            card_number=None,
            language_preference="BOTH",
            slab_tier="PSA_10",
            tab_name="ACTIVE",
            keywords="Charizard [1st Edition] #4 PSA 10",
            day_range=30,
        )

        self.assertEqual(metrics.total_listings, 1)
        self.assertEqual(metrics.avg_listing_price, 750000.0)
        self.assertIn("#4", metrics.top_rows[0].title)
        self.assertNotIn("#017", metrics.top_rows[0].title)



if __name__ == "__main__":
    main()
