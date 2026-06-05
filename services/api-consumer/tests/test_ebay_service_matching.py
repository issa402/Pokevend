import unittest
from pathlib import Path

from repositories.ebay_repo import _env_file_candidates, _marketplace_query
from services.ebay_service import EbayService, _matches_card_number, _matches_set


class EnvFileCandidatesTest(unittest.TestCase):
    def test_shallow_container_path_does_not_require_missing_parent(self):
        candidates = _env_file_candidates(Path("/app/repositories/ebay_repo.py"))

        self.assertEqual(candidates, [Path("/app/.env")])


class FakeRepo:
    async def search_listings(self, query, limit=200, max_pages=1):
        return {
            "itemSummaries": [
                {
                    "title": "2009 POKEMON RUMBLE #12 LUCARIO PSA 9",
                    "price": {"value": 3499.99},
                    "itemWebUrl": "https://www.ebay.com/itm/1",
                    "itemId": "1",
                },
                {
                    "title": "2025 Pokemon Mega Evolution Mega Lucario EX PSA 9",
                    "price": {"value": 24.97},
                    "itemWebUrl": "https://www.ebay.com/itm/2",
                    "itemId": "2",
                },
                {
                    "title": "PSA 8 Lucario 12/16 Pokemon Rumble NM-MT Graded Pokemon Card",
                    "price": {"value": 499.99},
                    "itemWebUrl": "https://www.ebay.com/itm/3",
                    "itemId": "3",
                },
            ]
        }


class FakePublisher:
    def __init__(self):
        self.messages = []

    async def publish(self, routing_key, payload):
        self.messages.append((routing_key, payload))


class EbayServiceMatchingTest(unittest.IsolatedAsyncioTestCase):
    async def test_scan_card_requires_set_tokens_for_slab_targets(self):
        publisher = FakePublisher()
        service = EbayService(FakeRepo(), publisher)

        listings = await service.scan_card(
            card_name="Lucario",
            external_card_id="ru1-12",
            set_name="Pokémon Rumble",
            asset_type="SLAB",
            slab_tier="PSA_9",
            publish=True,
        )

        self.assertEqual([listing.listing_id for listing in listings], ["1"])
        self.assertEqual(listings[0].price, 3499.99)
        self.assertEqual(len(publisher.messages), 1)

    async def test_scan_card_supports_lower_bgs_decimal_grades(self):
        class BgsRepo:
            async def search_listings(self, query, limit=200, max_pages=1):
                return {
                    "itemSummaries": [{
                        "title": "2009 POKEMON PROMOS RUMBLE #12 LUCARIO BGS 7.5",
                        "price": {"value": 1000},
                        "itemWebUrl": "https://www.ebay.com/itm/4",
                        "itemId": "4",
                    }]
                }

        service = EbayService(BgsRepo(), FakePublisher())
        listings = await service.scan_card(
            card_name="Lucario",
            external_card_id="ru1-12",
            set_name="Pokémon Rumble",
            asset_type="SLAB",
            slab_tier="BGS_7_5",
        )

        self.assertEqual(len(listings), 1)
        self.assertEqual(listings[0].slab_tier, "BGS_7_5")

    async def test_scan_card_uses_scrapling_when_browse_results_filter_to_empty(self):
        class WrongBrowseRepo:
            async def search_listings(self, query, limit=200, max_pages=1):
                return {
                    "itemSummaries": [{
                        "title": "2025 Pokemon Mega Evolution Mega Lucario EX CGC 10",
                        "price": {"value": 24.97},
                        "itemWebUrl": "https://www.ebay.com/itm/wrong",
                        "itemId": "wrong",
                    }]
                }

        import services.ebay_service as ebay_service
        original = ebay_service._scrape_ebay_active_listings
        try:
            ebay_service._scrape_ebay_active_listings = lambda query: [{
                "title": "2004 POKEMON EX TEAM ROCKET RETURNS REVERSE HOLO AZUMARILL CGC 10 GEM MINT",
                "price": {"value": 999.99},
                "itemWebUrl": "https://www.ebay.com/itm/azumarill",
                "itemId": "azumarill",
            }]
            service = EbayService(WrongBrowseRepo(), FakePublisher())
            listings = await service.scan_card(
                card_name="Azumarill",
                external_card_id="ex7-1",
                set_name="EX Team Rocket Returns",
                asset_type="SLAB",
                slab_tier="CGC_10",
            )
        finally:
            ebay_service._scrape_ebay_active_listings = original

        self.assertEqual(len(listings), 1)
        self.assertEqual(listings[0].listing_id, "azumarill")
        self.assertEqual(listings[0].slab_tier, "CGC_10")
        self.assertEqual(listings[0].price, 999.99)

    async def test_scan_card_uses_scrapling_when_browse_api_fails(self):
        class FailingBrowseRepo:
            async def search_listings(self, query, limit=200, max_pages=1):
                raise RuntimeError("401 Unauthorized")

        import services.ebay_service as ebay_service
        original = ebay_service._scrape_ebay_active_listings
        try:
            ebay_service._scrape_ebay_active_listings = lambda query: [{
                "title": "2006 POKEMON EX LEGEND MAKER #26 SPINDA-REVERSE FOIL PSA 7",
                "price": {"value": 125.00},
                "itemWebUrl": "https://www.ebay.com/itm/spinda-psa7",
                "itemId": "spinda-psa7",
            }]
            publisher = FakePublisher()
            service = EbayService(FailingBrowseRepo(), publisher)
            listings = await service.scan_card(
                card_name="Spinda",
                external_card_id="ex12-26",
                set_name="EX Legend Maker",
                card_number="26",
                asset_type="SLAB",
                slab_tier="PSA_7",
                publish=True,
            )
        finally:
            ebay_service._scrape_ebay_active_listings = original

        self.assertEqual([listing.listing_id for listing in listings], ["spinda-psa7"])
        self.assertEqual(listings[0].slab_tier, "PSA_7")
        self.assertEqual(len(publisher.messages), 1)

    async def test_scan_card_requires_card_number_when_available(self):
        class MixedNumberRepo:
            async def search_listings(self, query, limit=200, max_pages=1):
                return {
                    "itemSummaries": [
                        {
                            "title": "PSA 10 Azumarill Holo 025/084 Team Rocket Returns PCG Pokemon Card Japanese",
                            "price": {"value": 12999},
                            "itemWebUrl": "https://www.ebay.com/itm/japanese",
                            "itemId": "japanese",
                        },
                        {
                            "title": "Team Rocket Returns - Azumarill 1/109 Holo - PSA 8",
                            "price": {"value": 74.01},
                            "itemWebUrl": "https://www.ebay.com/itm/psa8",
                            "itemId": "psa8",
                        },
                    ]
                }

        service = EbayService(MixedNumberRepo(), FakePublisher())
        listings = await service.scan_card(
            card_name="Azumarill",
            external_card_id="ex7-1",
            set_name="Team Rocket Returns",
            card_number="1",
            asset_type="SLAB",
            slab_tier="PSA_8",
        )

        self.assertEqual([listing.listing_id for listing in listings], ["psa8"])

    async def test_scan_card_does_not_add_language_word_to_marketplace_query(self):
        class CapturingRepo:
            def __init__(self):
                self.query = None

            async def search_listings(self, query, limit=200, max_pages=1):
                self.query = query
                return {"itemSummaries": []}

        repo = CapturingRepo()
        service = EbayService(repo, FakePublisher())
        await service.scan_card(
            card_name="Azumarill",
            external_card_id="ex7-1",
            set_name="Team Rocket Returns",
            card_number="1",
            asset_type="SLAB",
            slab_tier="PSA_8",
            language_preference="ENGLISH",
        )

        self.assertNotIn("English", repo.query)
        self.assertIn("Azumarill", repo.query)
        self.assertIn("Team Rocket Returns", repo.query)
        self.assertIn("PSA 8", repo.query)


    async def test_import_listing_text_parses_copied_ebay_results_and_publishes(self):
        copied = """
        2004 POKEMON EX TEAM ROCKET RETURNS, REVERSE HOLO AZUMARILL CGC 10 GEM MINT Image 1 of 2
        2004 POKEMON EX TEAM ROCKET RETURNS, REVERSE HOLO AZUMARILL CGC 10 GEM MINT
        New (Other)
        $999.99
        Located in United States

        Team Rocket Returns - Azumarill 1/109 Holo - PSA 8 Image 1 of 2
        Team Rocket Returns - Azumarill 1/109 Holo - PSA 8
        New (Other)
        $74.01
        18 watchers

        2004 POKEMON EX TEAM ROCKET RETURNS 1 AZUMARILL JAMES THEME DECK PSA 3 VG
        New (Other)
        $35.00
        """
        publisher = FakePublisher()
        service = EbayService(FakeRepo(), publisher)

        listings = await service.import_listing_text(
            copied,
            card_name="Azumarill",
            external_card_id="ex7-1",
            set_name="EX Team Rocket Returns",
            card_number="1",
            asset_type="SLAB",
            language_preference="ENGLISH",
        )

        tiers = {listing.slab_tier for listing in listings}
        self.assertEqual(len(listings), 3)
        self.assertIn("CGC_10", tiers)
        self.assertIn("PSA_8", tiers)
        self.assertIn("PSA_3", tiers)
        self.assertEqual(len(publisher.messages), 3)
        self.assertTrue(all(listing.listing_id.startswith("manual:") for listing in listings))

    async def test_import_listing_text_keeps_card_number_filter(self):
        copied = """
        PSA 10 Azumarill Holo 025/084 Team Rocket Returns PCG Pokemon Card Japanese
        $12999.00
        Team Rocket Returns - Azumarill 1/109 Holo - PSA 8
        $74.01
        """
        service = EbayService(FakeRepo(), FakePublisher())

        listings = await service.import_listing_text(
            copied,
            card_name="Azumarill",
            external_card_id="ex7-1",
            set_name="Team Rocket Returns",
            card_number="1",
            asset_type="SLAB",
            language_preference="ENGLISH",
        )

        self.assertEqual([listing.slab_tier for listing in listings], ["PSA_8"])


    async def test_scan_card_uses_card_number_query_variant_for_sparse_slabs(self):
        class SparseSlabRepo:
            def __init__(self):
                self.queries = []

            async def search_listings(self, query, limit=200, max_pages=1):
                self.queries.append(query)
                if "26" not in query:
                    return {"itemSummaries": []}
                return {"itemSummaries": [{
                    "title": "Spinda Pokemon 2006 EX Legend Maker 26/92 Rare  - CGC 9 MINT",
                    "price": {"value": 15.99},
                    "itemWebUrl": "https://www.ebay.com/itm/spinda-cgc9",
                    "itemId": "spinda-cgc9",
                }]}

        repo = SparseSlabRepo()
        service = EbayService(repo, FakePublisher())
        listings = await service.scan_card(
            card_name="Spinda",
            external_card_id="ex12-26",
            set_name="EX Legend Maker",
            card_number="26",
            asset_type="SLAB",
            slab_tier="CGC_9",
        )

        self.assertEqual(len(listings), 1)
        self.assertEqual(listings[0].slab_tier, "CGC_9")
        self.assertTrue(any("26" in query for query in repo.queries))


class SetMatchingTest(unittest.TestCase):
    def test_marketplace_query_does_not_append_pokemon_card_to_rich_slab_query(self):
        self.assertEqual(_marketplace_query("Spinda 26/92 EX Legend Maker CGC 9"), "Spinda 26/92 EX Legend Maker CGC 9")
        self.assertEqual(_marketplace_query("Spinda #26 EX Legend Maker PSA 7"), "Spinda #26 EX Legend Maker PSA 7")
        self.assertEqual(_marketplace_query("Spinda"), "Spinda pokemon card")

    def test_matches_set_normalizes_pokemon_accent(self):
        self.assertTrue(_matches_set("2009 Pokemon Rumble Lucario PSA 9", "Pokémon Rumble"))

    def test_matches_set_rejects_unrelated_same_character_listing(self):
        self.assertFalse(_matches_set("2025 Pokemon Mega Evolution Mega Lucario EX PSA 9", "Pokémon Rumble"))

    def test_matches_card_number_accepts_missing_number_but_rejects_conflict(self):
        self.assertTrue(_matches_card_number("2004 POKEMON EX TEAM ROCKET RETURNS REVERSE HOLO AZUMARILL CGC 10", "1"))
        self.assertTrue(_matches_card_number("Team Rocket Returns - Azumarill 1/109 Holo - PSA 8", "1"))
        self.assertFalse(_matches_card_number("PSA 10 Azumarill Holo 025/084 Team Rocket Returns PCG Pokemon Card Japanese", "1"))


if __name__ == "__main__":
    unittest.main()
