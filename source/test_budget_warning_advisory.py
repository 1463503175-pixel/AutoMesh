# coding: utf-8
"""Offline ACT warning-policy tests. Never connects to ANSYS."""
import unittest
from assets.warning_refinement import _check_budget_warning_approval, MAX_CELL_PROXY


class BudgetWarningApprovalTests(unittest.TestCase):
    def test_legacy_within_threshold_needs_no_new_field(self):
        self.assertFalse(_check_budget_warning_approval(MAX_CELL_PROXY, {}))
        self.assertFalse(_check_budget_warning_approval(119900, {'schema_version': 1}))

    def test_over_threshold_must_have_explicit_true(self):
        for receipt in ({}, {'budget_warning_acknowledged': False},
                        {'budget_warning_acknowledged': 'true'}, None):
            with self.subTest(receipt=receipt):
                with self.assertRaisesRegex(RuntimeError, '额外人工确认'):
                    _check_budget_warning_approval(120217, receipt)

    def test_acknowledged_over_threshold_is_advisory(self):
        self.assertTrue(_check_budget_warning_approval(120217,
                        {'budget_warning_acknowledged': True}))

    def test_invalid_live_proxy_is_not_approved(self):
        for proxy in (float('nan'),float('inf'),-1):
            with self.subTest(proxy=proxy):
                with self.assertRaises(RuntimeError):
                    _check_budget_warning_approval(proxy,{'budget_warning_acknowledged': True})


if __name__ == '__main__':
    unittest.main()
