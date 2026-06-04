from unittest import IsolatedAsyncioTestCase
from unittest.mock import AsyncMock, patch

from seller_hub_research_batch import ResearchTarget, research_metric_with_retry


class SellerHubBatchRetryTest(IsolatedAsyncioTestCase):
    async def test_retries_transient_browser_failure(self):
        target = ResearchTarget("Charizard #4", "Base Set", "PSA_10", None)
        metric = object()
        with patch(
            "seller_hub_research_batch.research_one_grade",
            new=AsyncMock(side_effect=[RuntimeError("network changed"), metric]),
        ) as research:
            with patch("seller_hub_research_batch.asyncio.sleep", new=AsyncMock()):
                result = await research_metric_with_retry(target, "ACTIVE", 30, True, attempts=2)

        self.assertIs(result, metric)
        self.assertEqual(research.await_count, 2)
