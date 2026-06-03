import json
import tempfile
from pathlib import Path
from unittest import TestCase, main
from unittest.mock import patch

from services.scrapling_sold_comps_batch import load_source_config, run_batch


class ScraplingSoldCompsBatchTest(TestCase):
    def write_config(self, payload):
        tmp = tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False)
        with tmp:
            json.dump(payload, tmp)
        return Path(tmp.name)

    def test_load_source_config_rejects_missing_required_fields(self):
        path = self.write_config({"sources": [{"name": "bad"}]})

        with self.assertRaises(ValueError):
            load_source_config(path)

    def test_run_batch_fetches_enabled_sources_and_dry_run_skips_repo_write(self):
        path = self.write_config({
            "sources": [
                {
                    "name": "armored-mewtwo-psa10",
                    "enabled": True,
                    "url": "https://example.com/comps",
                    "marketplace": "fixture",
                    "cardName": "Armored Mewtwo",
                    "setName": "SM Promos",
                    "externalCardId": "smp-SM228",
                    "grader": "PSA",
                    "grade": "10",
                    "slabTier": "PSA_10",
                    "selectors": {
                        "row": ".sold-row",
                        "title": ".title::text",
                        "price": ".price::text",
                        "soldAt": ".sold::text",
                        "link": "a"
                    }
                },
                {"name": "disabled", "enabled": False}
            ]
        })

        fake_comps = [{"card_name": "Armored Mewtwo", "sold_price": 150.0}]
        with patch("services.scrapling_sold_comps_batch.ScraplingSoldCompExtractor") as extractor_cls, \
             patch("services.scrapling_sold_comps_batch.SlabCompRepo") as repo_cls:
            extractor_cls.return_value.fetch.return_value = fake_comps
            summary = run_batch(path, dry_run=True)

        self.assertEqual(summary["sourcesSeen"], 2)
        self.assertEqual(summary["sourcesFetched"], 1)
        self.assertEqual(summary["compsParsed"], 1)
        self.assertEqual(summary["compsInserted"], 0)
        repo_cls.return_value.upsert_many.assert_not_called()


if __name__ == "__main__":
    main()
