from unittest import TestCase

from services.vendor_opportunity_alerts import vendor_alert_message


class VendorOpportunityAlertTest(TestCase):
    def test_message_explains_exact_listing_economics(self):
        message = vendor_alert_message("Charizard #4", "PSA_10", 100.0, 180.0, 80.0, 70.18)

        self.assertIn("Charizard #4 PSA_10", message)
        self.assertIn("$100.00", message)
        self.assertIn("$80.00", message)
        self.assertIn("70.2%", message)
