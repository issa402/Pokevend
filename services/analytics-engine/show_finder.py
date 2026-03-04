"""
============================================================
PokémonTool — Show Finder
============================================================
Fetches upcoming Pokémon TCG shows and events from Eventbrite
and stores them in MongoDB (synced to PostgreSQL by this service).
Runs daily at midnight UTC.
============================================================
"""

import os
import logging
from datetime import datetime, timedelta

import requests

log = logging.getLogger(__name__)


class ShowFinder:
    """Fetches and stores upcoming Pokémon TCG events."""

    def __init__(self, db):
        self.db        = db
        self.shows_col = db["shows"]

    def run(self):
        """Refresh upcoming shows from Eventbrite API."""
        log.info("Refreshing upcoming Pokémon TCG shows...")

        token = os.getenv("EVENTBRITE_TOKEN")
        if not token:
            log.warning("EVENTBRITE_TOKEN not set — skipping show fetch")
            return

        try:
            resp = requests.get(
                "https://www.eventbriteapi.com/v3/events/search/",
                headers={"Authorization": f"Bearer {token}"},
                params={
                    "q":              "pokemon trading card",
                    "sort_by":        "date",
                    "expand":         "venue",
                    "start_date.range_start": datetime.utcnow().isoformat() + "Z",
                    "start_date.range_end":   (datetime.utcnow() + timedelta(days=90)).isoformat() + "Z",
                },
                timeout=10,
            )
            resp.raise_for_status()
            events = resp.json().get("events", [])

            inserted = 0
            for ev in events:
                venue  = ev.get("venue", {})
                doc = {
                    "eventbriteId": ev["id"],
                    "name":         ev.get("name", {}).get("text", ""),
                    "venueName":    venue.get("name", ""),
                    "city":         venue.get("address", {}).get("city", ""),
                    "state":        venue.get("address", {}).get("region", ""),
                    "zipCode":      venue.get("address", {}).get("postal_code", ""),
                    "startDate":    ev.get("start", {}).get("utc"),
                    "endDate":      ev.get("end",   {}).get("utc"),
                    "eventUrl":     ev.get("url", ""),
                    "description":  ev.get("description", {}).get("text", "")[:500],
                    "fetchedAt":    datetime.utcnow(),
                }
                # Upsert so we don't create duplicates on repeated runs
                self.shows_col.update_one(
                    {"eventbriteId": ev["id"]},
                    {"$set": doc},
                    upsert=True,
                )
                inserted += 1

            log.info(f"Show finder: {inserted} shows updated from Eventbrite")
        except Exception as e:
            log.error(f"Show finder failed: {e}", exc_info=True)
