# -*- coding: utf-8 -*-
"""Bounded, one-click low-quality prioritization; no real ACT required."""
import unittest
from types import SimpleNamespace as NS
from test_skew_range_r74 import FakeModel, unit
import math


def run(vals, low, high):
    model=FakeModel(vals)
    ctx=dict(Model=model, math=math, utext=str,
        job={'job_id':'e'*32}, cancellation_checkpoint=lambda:None,
        SelectionType=NS(MeshElement='mesh-element'),
        SelectionCriterionType=NS(Skewness='skewness', ElementQuality='skewness'),
        SelectionOperatorType=NS(RangeInclude='between'),
        SelectionActionType=NS(Filter='filter'),
        GeometryDefineByType=NS(Worksheet='worksheet'),
        SelectionTypeEnum=NS(MeshElements='mesh-elements'))
    exec(unit,ctx)
    return model,ctx['mark_threshold_element_quality_elements'](low,high)


class AdaptiveEQTests(unittest.TestCase):
    def test_broad_range_prioritizes_low_quality_without_allocation_above_cap(self):
        vals={i:(0.15+(i/10000.0)*0.60) for i in range(10000)}
        model, result=run(vals,0.15,0.8)
        self.assertEqual(result['status'],'created')
        self.assertTrue(result['auto_limited'])
        self.assertLessEqual(result['worksheet_count'],4096)
        self.assertGreater(result['worksheet_count'],0)
        self.assertLess(result['range_max'],result['requested_range_max'])
        self.assertEqual(len(model.created),1)
        self.assertIn('未处理', result['message'])
    def test_narrow_range_still_uses_original_behavior(self):
        model,result=run({1:.15,2:.16,3:.18},.15,.17)
        self.assertEqual(result['status'],'created')
        self.assertFalse(result['auto_limited'])
        self.assertEqual(result['worksheet_count'],2)
    def test_no_safe_low_quality_prefix_fails_closed(self):
        model,result=run({i:.15 for i in range(6000)},.15,.8)
        self.assertEqual(result['status'],'too_many_elements')
        self.assertEqual(len(model.created),0)

if __name__=='__main__': unittest.main()
