from unittest import TestCase

from seller_hub_research_batch import normalize_tabs


class SellerHubBatchTest(TestCase):
    def test_normalize_tabs_keeps_supported_unique_tabs(self):
        self.assertEqual(normalize_tabs("active,SOLD,active"), ["ACTIVE", "SOLD"])

    def test_normalize_tabs_rejects_unsupported_tab(self):
        with self.assertRaises(ValueError):
            normalize_tabs("ACTIVE,COMPLETED")
