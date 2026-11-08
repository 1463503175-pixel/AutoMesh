# -*- coding: utf-8 -*-
"""Regression for actual R7.4 ACT failure: unitless Skewness Worksheet
values must be numeric IConvertible, not Ansys.Core.Units.Quantity.
"""
import unittest
from types import SimpleNamespace as NS
from test_skew_range_r74 import exercise


class UnitlessWorksheetRegression(unittest.TestCase):
    @staticmethod
    def quantity_is_not_allowed(value):
        raise TypeError("Cannot convert Ansys.Core.Units.Quantity to System.IConvertible: %r" % value)

    def test_range_include_uses_plain_numbers(self):
        model, result = exercise(
            {10: 0.8999, 11: 0.90, 12: 0.9731728, 13: 1.0},
            overrides={'Quantity': self.quantity_is_not_allowed},
        )
        self.assertEqual(result['status'], 'created')
        self.assertEqual(result['worksheet_count'], 3)
        self.assertEqual(model.created[0].Location.Ids, [11, 12, 13])
        row = model.created[0].GenerationCriteria[0]
        self.assertIs(type(row.LowerBound), float)
        self.assertIs(type(row.UpperBound), float)
        self.assertEqual((row.LowerBound, row.UpperBound), (.9, 1.0))

    def test_two_row_fallback_uses_plain_numbers(self):
        model, result = exercise(
            {10: .88, 11: .9, 12: .94, 13: .97317}, .9, .95,
            overrides={
                'Quantity': self.quantity_is_not_allowed,
                'SelectionOperatorType': NS(GreaterThanOrEqual='gte', LessThanOrEqual='lte'),
            },
        )
        self.assertEqual(result['status'], 'created')
        self.assertEqual(model.created[0].Location.Ids, [11, 12])
        a, b = model.created[0].GenerationCriteria
        self.assertIs(type(a.Value), float)
        self.assertIs(type(b.Value), float)
        self.assertEqual((a.Value, b.Value), (.9, .95))

    def test_skewness_marker_no_effect_on_mesh(self):
        model, result = exercise({1: .97317}, overrides={'Quantity': self.quantity_is_not_allowed})
        self.assertEqual(result['status'], 'created')
        self.assertEqual(model.Mesh.Elements, 1)
        self.assertEqual(len(model.created), 1)

if __name__ == '__main__':
    unittest.main()
