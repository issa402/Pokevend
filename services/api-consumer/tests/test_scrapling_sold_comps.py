from datetime import timezone
from unittest import TestCase, main
from unittest.mock import patch

from services import scrapling_sold_comps
from services.scrapling_sold_comps import (
    ScraplingSoldCompExtractor,
    SoldCompContext,
    SoldCompSelectorConfig,
)


class FakeSelection:
    def __init__(self, value="", attrib=None):
        self._value = value
        self.attrib = attrib or {}

    def get(self):
        return self._value


class FakeSelectorList:
    def __init__(self, items):
        self.items = items

    def __bool__(self):
        return bool(self.items)

    def __getitem__(self, index):
        return self.items[index]

    def get(self):
        return self.items[0].get()


class FakeRow:
    def css(self, selector):
        values = {
            ".title::text": FakeSelectorList([FakeSelection("Armored Mewtwo PSA 10 cert #123456")]),
            ".price::text": FakeSelectorList([FakeSelection("$125.50")]),
            ".sold::text": FakeSelectorList([FakeSelection("May 30, 2026")]),
            "a": FakeSelectorList([FakeSelection(attrib={"href": "/sold/123"})]),
        }
        return values.get(selector, FakeSelectorList([]))


class FakePage:
    def css(self, selector):
        if selector == ".sold-row":
            return [FakeRow()]
        return []


class ScraplingSoldCompsTest(TestCase):
    def test_fetch_extracts_sold_comp_link_from_selector_list(self):
        with patch.object(scrapling_sold_comps.Fetcher, "get", return_value=FakePage()):
            comps = ScraplingSoldCompExtractor().fetch(
                "https://example.com/comps",
                SoldCompSelectorConfig(
                    row_selector=".sold-row",
                    title_selector=".title::text",
                    price_selector=".price::text",
                    sold_at_selector=".sold::text",
                    link_selector="a",
                ),
                SoldCompContext(
                    card_name="Armored Mewtwo",
                    set_name="SM Promos",
                    external_card_id="smp-SM228",
                    grader="PSA",
                    grade="10",
                    slab_tier="PSA_10",
                    marketplace="fixture",
                ),
            )

        self.assertEqual(len(comps), 1)
        self.assertEqual(comps[0]["sold_price"], 125.50)
        self.assertEqual(comps[0]["listing_url"], "https://example.com/sold/123")
        self.assertEqual(comps[0]["cert_number"], "123456")
        self.assertEqual(comps[0]["sold_at"].tzinfo, timezone.utc)


if __name__ == "__main__":
    main()
