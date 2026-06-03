from unittest import TestCase, main

from services.pricecharting_market import (
    parse_big_mover_rows,
    parse_grade_prices,
    money_to_float,
)


BIG_MOVERS_HTML = """
<table>
  <tr><th>Title</th><th>console</th><th>Loose Price</th><th>Change</th></tr>
  <tr>
    <td><a href="/game/pokemon-promo/armored-mewtwo-sm228">Armored Mewtwo #SM228</a></td>
    <td>Pokemon Promo</td><td>$167.09</td><td>+ $7.93</td>
  </tr>
  <tr>
    <td><a href="/game/pokemon-base-set/booster-box">Booster Box</a></td>
    <td>Pokemon Base Set</td><td>$10,000.00</td><td>+ $500.00</td>
  </tr>
</table>
"""

CARD_HTML = """
<table id="price_data">
  <tr><th>Ungraded</th><th>Grade 7</th><th>Grade 8</th><th>Grade 9</th><th>Grade 9.5</th><th>PSA 10</th></tr>
  <tr><td><span class="price">$167.09</span></td><td><span class="price">$192.93</span></td><td><span class="price">$277.50</span></td><td><span class="price">$723.69</span></td><td><span class="price">$796.00</span></td><td><span class="price">$8,000.00</span></td></tr>
</table>
"""


class PriceChartingMarketTest(TestCase):
    def test_money_to_float_accepts_ebay_numeric_strings(self):
        self.assertEqual(money_to_float("10875.05"), 10875.05)
        self.assertEqual(money_to_float("$10,875.05"), 10875.05)

    def test_parse_big_movers_skips_sealed_products(self):
        rows = parse_big_mover_rows(BIG_MOVERS_HTML)

        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].title, "Armored Mewtwo #SM228")
        self.assertEqual(rows[0].loose_price, 167.09)
        self.assertEqual(rows[0].change_amount, 7.93)
        self.assertEqual(rows[0].url, "https://www.pricecharting.com/game/pokemon-promo/armored-mewtwo-sm228")

    def test_parse_grade_prices_extracts_slab_tiers(self):
        prices = parse_grade_prices(CARD_HTML)

        self.assertEqual(prices["GRADE_9"], 723.69)
        self.assertEqual(prices["GRADE_9_5"], 796.00)
        self.assertEqual(prices["PSA_10"], 8000.00)


if __name__ == "__main__":
    main()
