# coding: utf-8
"""v0.9.6 local sizing and 32GiB guard regressions (pure Python, no ANSYS)."""
import unittest
from assets.geometry_rules import classify, evaluate_refinement_budget

class ResourceBudgetTests(unittest.TestCase):
    def test_all_diameter_bands_are_independent(self):
        faces = []
        for index, diameter in enumerate((1.8, 5., 15.)):
            radius=diameter/2
            faces.append({'id':index+1, 'area':500, 'body':1, 'type':'CylindricalSurface',
                          'full_circle_radii':[radius,radius]})
        groups, evidence = classify({'geometry_unit':'mm', 'faces':faces},12)
        cylinders = [g for g in groups if g['category']=='circular_cylindrical']
        self.assertEqual(len(cylinders),3)
        self.assertEqual(sum(len(g['ids']) for g in cylinders),3)
        self.assertEqual(len(evidence['cylinder_diameter_groups']),3)
        self.assertLess(cylinders[0]['mm'],cylinders[-1]['mm'])

    def test_after_override_recalculate_and_refuse_32gib_risk(self):
        state={'geometry_unit':'mm','faces':[{'id':1,'area':850000},{'id':2,'area':850000}]}
        groups=[{'category':'circular_cylindrical','ids':[1], 'mm':4},
                {'category':'named_inlet','ids':[2], 'mm':4}]
        baseline=evaluate_refinement_budget(state,groups)
        self.assertLess(baseline['surface_proxy'],1500000)
        with self.assertRaisesRegex(ValueError,'32GB'):
            evaluate_refinement_budget(state,groups,{'circular_cylindrical':1,'named_inlet':1})

    def test_face_is_charged_once_even_in_two_groups(self):
        state={'geometry_unit':'mm','faces':[{'id':1,'area':850000}]}
        groups=[{'category':'circular_cylindrical','ids':[1], 'mm':1},
                {'category':'named_inlet','ids':[1], 'mm':1}]
        b=evaluate_refinement_budget(state,groups)
        self.assertEqual(b['surface_proxy'],850000)
        self.assertEqual(b['sizing_groups'][0]['face_count'],1)
        self.assertEqual(len(b['sizing_groups']),1)

    def test_disabled_and_excluded_groups_do_not_consume_budget(self):
        state={'geometry_unit':'mm','faces':[{'id':1,'area':850000},{'id':2,'area':850000}]}
        groups=[{'category':'circular_cylindrical','ids':[1], 'mm':1},
                {'category':'named_inlet','ids':[2], 'mm':1}]
        self.assertEqual(evaluate_refinement_budget(state,groups,disabled=['named_inlet'])['surface_proxy'],850000)
        self.assertEqual(evaluate_refinement_budget(state,groups,excluded_ids=[1])['surface_proxy'],850000)

    def test_missing_face_area_is_not_forged_into_safe_result(self):
        with self.assertRaisesRegex(ValueError,'unreadable area'):
            evaluate_refinement_budget({'geometry_unit':'mm','faces':[{'id':4}]},
                                       [{'category':'named_inlet','ids':[4],'mm':1}])

    def test_mm_meters_conversion_applies_to_budget(self):
        state={'geometry_unit':'m','faces':[{'id':1,'area':0.1}]}
        b=evaluate_refinement_budget(state,[{'category':'named_inlet','ids':[1],'mm':2}])
        self.assertEqual(b['surface_proxy'],25000)

    def test_cannot_coarsen_repeated_gap_override(self):
        state={'geometry_unit':'mm','faces':[{'id':1,'area':50}]}
        groups=[{'category':'narrow_parallel','ids':[1],'mm':1.2,'gap_mm_min':3}]
        with self.assertRaisesRegex(ValueError,'too coarse'):
            evaluate_refinement_budget(state,groups,{'narrow_parallel':2})

if __name__=='__main__': unittest.main()
