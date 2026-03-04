"""
============================================================
PokémonTool — News Scanner
============================================================
Scrapes Pokémon TCG news sites to detect events that could
cause price movements (new set reveals, tournament bans, etc.).

Sources scanned:
  - PokeGuardian (pokeguardian.com)
  - LimitlessTCG (limitlesstcg.com)
  - Pokebeach (pokebeach.com)

When a major card is mentioned in a news headline, correlates
it with current trending data and logs an annotation to MongoDB.
These annotations appear on price history charts in the UI.
============================================================
"""

import logging
import re
from datetime import datetime, timedelta
from typing import List, Tuple

import requests
from bs4 import BeautifulSoup

log = logging.getLogger(__name__)

# News sources to scan — add more as needed
NEWS_SOURCES = [
    {
        "name":          "PokeGuardian",
        "url":           "https://pokeguardian.com/",
        "article_sel":   "article.post",       # CSS selector for article elements
        "title_sel":     "h2.entry-title",
        "link_sel":      "h2.entry-title a",
    },
    {
        "name":          "Pokebeach",
        "url":           "https://www.pokebeach.com/",
        "article_sel":   "article",
        "title_sel":     "h2",
        "link_sel":      "h2 a",
    },
]

# Keywords that suggest a card price may be affected
PRICE_IMPACT_KEYWORDS = [
    "ban", "banned", "unbanned",
    "reprint", "reprinted",
    "revealed", "reveal",
    "tournament", "world championship",
    "rotation", "rotated",
    "alt art", "secret rare", "special illustration",
    "limited", "exclusive",
    "sold out", "sold for",
]


class NewsScanner:
    """Scrapes Pokémon TCG news and annotates relevant price events."""

    def __init__(self, db):
        self.db           = db
        self.news_col     = db["news_events"]    # Stores scanned headlines
        self.cards_col    = db["cards"]           # Cards collection to annotate

    def run(self):
        """Scan all configured news sources for price-relevant articles."""
        log.info("Scanning Pokémon TCG news sources...")
        try:
            total_articles = 0
            for source in NEWS_SOURCES:
                articles = self._scrape_source(source)
                for article in articles:
                    self._process_article(article, source["name"])
                    total_articles += 1
            log.info(f"News scan complete — processed {total_articles} articles")
        except Exception as e:
            log.error(f"News scan failed: {e}", exc_info=True)

    def _scrape_source(self, source: dict) -> List[dict]:
        """
        Scrapes headlines from a single news source.
        Returns a list of {title, url, source} dicts.
        """
        try:
            resp = requests.get(
                source["url"],
                headers={"User-Agent": "Mozilla/5.0 (PokémonTool news scanner)"},
                timeout=10,
            )
            resp.raise_for_status()
        except Exception as e:
            log.warning(f"Could not reach {source['name']}: {e}")
            return []

        soup     = BeautifulSoup(resp.text, "lxml")
        articles = soup.select(source["article_sel"])
        results  = []

        for article in articles[:10]:  # Limit to top 10 articles per source
            title_el = article.select_one(source["title_sel"])
            link_el  = article.select_one(source["link_sel"])

            if title_el and link_el:
                results.append({
                    "title": title_el.get_text(strip=True),
                    "url":   link_el.get("href", ""),
                })

        log.debug(f"{source['name']}: found {len(results)} articles")
        return results

    def _process_article(self, article: dict, source_name: str):
        """
        Checks if an article title has already been processed and if it
        contains price-relevant keywords. If so, stores an annotation.
        """
        # Avoid re-processing the same article
        existing = self.news_col.find_one({"url": article["url"]})
        if existing:
            return

        title = article["title"].lower()
        is_price_relevant = any(kw in title for kw in PRICE_IMPACT_KEYWORDS)

        # Store the article regardless — so we don't scrape it again
        self.news_col.insert_one({
            "title":       article["title"],
            "url":         article["url"],
            "source":      source_name,
            "isPriceRelevant": is_price_relevant,
            "scannedAt":   datetime.utcnow(),
        })

        if is_price_relevant:
            log.info(f"Price-relevant article found from {source_name}: {article['title'][:80]}")
            # In the future, extract card names from title and annotate price history
