"""Find and persist wholesale slab opportunities from active listings."""

from __future__ import annotations

import json
import logging
from decimal import Decimal

from repositories.listing_repo import ListingRepo
from slab_opportunity_scorer import score_listing

logger = logging.getLogger(__name__)


class SlabOpportunityFinder:
    def __init__(self, listing_repo: ListingRepo):
        self.listing_repo = listing_repo

    def run(self, hours: int = 24, limit: int = 200) -> list[dict]:
        listings = self.listing_repo.get_recent_slab_listings(hours=hours, limit=limit)
        saved: list[dict] = []
        for listing in listings:
            external_card_id = listing.get("external_card_id")
            card_name = listing.get("card_name") or ""
            slab_tier = listing.get("slab_tier") or ""
            comps = self.listing_repo.get_slab_comps(external_card_id, card_name, slab_tier)
            score = score_listing(listing, comps)
            if score is None:
                continue
            record = score.to_record()
            record["evidence"] = json.dumps(record["evidence"], sort_keys=True)
            self.listing_repo.upsert_slab_opportunity(record)
            saved.append(score.to_record())

        logger.info("Slab opportunity finder: scored %s listings, saved %s opportunities", len(listings), len(saved))
        return saved
