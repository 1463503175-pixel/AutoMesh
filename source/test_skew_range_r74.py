# -*- coding: utf-8 -*-
"""R7.4: verify exact two-ended skewness selection independently of native warnings."""
import ast
import math
import os
import unittest
from types import SimpleNamespace as NS

WORKER = os.path.join(os.path.dirname(__file__), 'assets', 'meshing_worker.py')
with open(WORKER, 'r', encoding='utf-8') as f:
    tree = ast.parse(f.read())
funcs = [n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name in
         ('_selection_enum', 'validate_skew_range', 'mark_threshold_quality_elements', 'mark_threshold_skew_elements', 'mark_threshold_element_quality_elements')]
assert len(funcs) == 5
unit = compile(ast.Module(body=funcs, type_ignores=[]), WORKER, 'exec')


class Criteria(list):
    def Add(self, ignored):
        self.append(NS(EntityType=None, Criterion=None, Operator=None,
                       Action=None, Value=None, LowerBound=None, UpperBound=None))


class CreatedSelection:
    def __init__(self, model):
        self.model = model
        self.Name = 'New'
        self.ScopingMethod = None
        self.GenerationCriteria = Criteria()
        self.Location = NS(Ids=[], SelectionType='mesh-elements')
        self.deleted = False
    def Generate(self):
        ids=[]
        for eid, skew in self.model.values.items():
            passed=True
            for row in self.GenerationCriteria:
                assert row.EntityType == 'mesh-element'
                assert row.Criterion == 'skewness'
                if row.Operator == 'between': test = float(row.LowerBound) <= skew <= float(row.UpperBound)
                elif row.Operator == 'gte': test = skew >= float(row.Value)
                elif row.Operator == 'lt': test = skew < float(row.Value)
                elif row.Operator == 'lte': test = skew <= float(row.Value)
                else: raise AssertionError('bad comparison: '+repr(row.Operator))
                passed = passed and test
            if passed: ids.append(eid)
        self.Location.Ids=ids
    def Delete(self):
        self.deleted = True
        self.model.created.remove(self)


class FakeModel:
    def __init__(self, values):
        self.values = values
        self.created = []
        self.Mesh = NS(Elements=len(values))
    def AddNamedSelection(self):
        obj = CreatedSelection(self)
        self.created.append(obj)
        return obj


def exercise(values, low=0.9, high=1.0, overrides=None):
    model = FakeModel(values)
    ctx = dict(Model=model, math=math, utext=str,
               job={'job_id': 'a'*32}, cancellation_checkpoint=lambda: None,
               Quantity=float,
               SelectionType=NS(MeshElement='mesh-element'),
               SelectionCriterionType=NS(Skewness='skewness'),
               SelectionOperatorType=NS(RangeInclude='between',GreaterThanOrEqual='gte', LessThanOrEqual='lte'),
               SelectionActionType=NS(Filter='filter'),
               GeometryDefineByType=NS(Worksheet='worksheet'),
               SelectionTypeEnum=NS(MeshElements='mesh-elements'))
    if overrides: ctx.update(overrides)
    exec(unit, ctx)
    return model, ctx['mark_threshold_skew_elements'](low, high)


class SkewRangeTests(unittest.TestCase):
    def test_default_90_to_100_finds_high_skew_despite_zero_quality_warnings(self):
        model, result = exercise({1: .88, 2: .90, 3: .92, 4: .97317, 5: .999})
        self.assertEqual(result['status'], 'created')
        self.assertEqual(result['worksheet_count'], 4)
        self.assertEqual(model.created[0].Location.Ids, [2, 3, 4, 5])
        self.assertEqual(len(model.created[0].GenerationCriteria), 1)
        self.assertEqual(result['selection_source'], 'named_selection_worksheet_skewness_range')
        self.assertTrue(model.created[0].Name.startswith('AutoMesh_Skew_900_1000'))

    def test_two_bounds_are_an_intersection_not_warning_threshold(self):
        model, result = exercise({1: .88, 2: .90, 3: .92, 4: .97317, 5: .999}, .90, .95)
        self.assertEqual(result['status'], 'created')
        self.assertEqual(result['worksheet_count'], 2)
        self.assertEqual(model.created[0].Location.Ids, [2,3])
        self.assertEqual(model.created[0].GenerationCriteria[0].Operator, 'between')
        self.assertEqual(len(model.created[0].GenerationCriteria), 1)

    def test_empty_range_deletes_only_new_group(self):
        model, result=exercise({1:.4,2:.6})
        self.assertEqual(result['status'], 'zero_elements')
        self.assertEqual(model.created, [])

    def test_user_range_validation(self):
        for low,high in [(1,1),(-.1,.95),(.9,1.1),(.95,.9),(math.nan,.9)]:
            with self.subTest(low=low, high=high):
                with self.assertRaises((ValueError,RuntimeError)):
                    exercise({1:.9},low,high)

    def test_old_act_fallback_two_rows_intersects_both_bounds(self):
        model,result=exercise({1:.88, 2:.90, 3:.93, 4:.973},.9,.95,
            {'SelectionOperatorType': NS(GreaterThanOrEqual='gte',LessThanOrEqual='lte')})
        self.assertEqual(result['status'],'created')
        self.assertEqual(model.created[0].Location.Ids,[2,3])
        self.assertEqual(model.created[0].GenerationCriteria[1].Action,'filter')

    def test_missing_upper_filter_rejects_without_creating_control(self):
        model=FakeModel({1:.91})
        ctx=dict(Model=model, math=math, utext=str, job={'job_id':'a'*32},
                 cancellation_checkpoint=lambda: None,Quantity=float,
                 SelectionType=NS(MeshElement='mesh-element'),
                 SelectionCriterionType=NS(Skewness='skewness'),
                 SelectionOperatorType=NS(GreaterThanOrEqualTo='gte', LessThanOrEqualTo='lte'),
                 SelectionActionType=NS(),GeometryDefineByType=NS(Worksheet='worksheet'))
        exec(unit,ctx)
        with self.assertRaises(RuntimeError):
            ctx['mark_threshold_skew_elements'](.9,.95)
        self.assertEqual(len(model.created),0)

    def test_safety_cap_keeps_old_mesh_and_aborts_oversized_named_selection(self):
        vals={i:.97 for i in range(50001)}
        model,result=exercise(vals)
        self.assertEqual(result['status'],'too_many_elements')
        self.assertEqual(model.created,[])
        self.assertEqual(model.Mesh.Elements,50001)


if __name__ == '__main__': unittest.main()
