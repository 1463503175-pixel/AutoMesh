# -*- coding: utf-8 -*-
"""Isolated ACT mocks: tests never import/launch ANSYS or edit a real model."""
import ast
import math
import time
import unittest
from pathlib import Path

SRC=Path(__file__).parent/'assets'/'meshing_worker.py'
tree=ast.parse(SRC.read_text(encoding='utf-8'))
node=next(x for x in tree.body if isinstance(x,ast.FunctionDef) and x.name=='collect_worksheet_quality_bounds')
code=compile(ast.Module(body=[node],type_ignores=[]),str(SRC),'exec')

class E: pass
class Row: pass
class Crit(list):
    def Add(self,_): self.append(Row())
class IDArray(list):
    @property
    def Count(self): return len(self)
class Target:
    def __init__(self,owner):
        self.owner=owner;self.GenerationCriteria=Crit();self.Location=E();self.Location.Ids=IDArray()
    def Generate(self):
        row=self.GenerationCriteria[0]
        if self.owner.raise_on_generate: raise RuntimeError('mock ACT failure')
        values=self.owner.skew if row.Criterion=='Skewness' else self.owner.orth
        if row.Operator=='GreaterThan':
            n=sum(v>row.Value for v in values)
        elif row.Operator=='LessThan':
            n=sum(v<row.Value for v in values)
        else: raise RuntimeError('Unexpected ACT operator')
        self.Location.Ids=IDArray(range(1,n+1))
    def Delete(self):
        if self.owner.raise_on_delete: raise RuntimeError('mock delete failed')
        self.owner.groups.remove(self)
class Model:
    def __init__(self,skew,orth=None):
        self.skew=skew;self.orth=orth or [0.4]*len(skew);self.Mesh=E();self.Mesh.Elements=len(skew)
        self.groups=['USER_EXISTING_GROUP'];self.raise_on_delete=False;self.raise_on_generate=False
    def AddNamedSelection(self):
        t=Target(self);self.groups.append(t);return t

def run(model,orth_available=True):
    selection=E();selection.MeshElement='MeshElement'
    criterion=E();criterion.Skewness='Skewness'
    if orth_available: criterion.OrthogonalQuality='OrthogonalQuality'
    op=E();op.GreaterThan='GreaterThan';op.LessThan='LessThan'
    geom=E();geom.Worksheet='Worksheet'
    env={'math':math,'time':time,'Model':model,'SelectionType':selection,
         'SelectionCriterionType':criterion,'SelectionOperatorType':op,
         'GeometryDefineByType':geom,'utext':str,
         'cancellation_checkpoint':lambda:None,'emit_progress':lambda *a:None,
         '_selection_enum':lambda enum,names,_purpose:next(getattr(enum,n) for n in names if hasattr(enum,n))}
    exec(code,env)
    stats,oq,diag=env['collect_worksheet_quality_bounds'](len(model.skew),{'skewness':{'max':max(model.skew)}})
    return stats,oq,diag

class WorksheetQualityProbeTests(unittest.TestCase):
    def test_exact_strict_counts_and_p95_bounds(self):
        skew=[.1]*90+[.901,.95,.951,.96,.98,.99,1.,.999,.955,.92]
        orth=[.01]+[.7]*99
        m=Model(skew,orth)
        stats,oq,diag=run(m)
        self.assertEqual(diag['status'],'completed',diag)
        self.assertEqual(diag['temporary_selection_cleanup'],'verified_deleted')
        self.assertEqual(m.groups,['USER_EXISTING_GROUP'])
        self.assertEqual(stats['above_0_90'],10)
        self.assertEqual(stats['above_0_95'],7)
        nearest_rank=sorted(skew)[math.ceil(.95*len(skew))-1]
        self.assertLessEqual(stats['p95_lower'],nearest_rank)
        self.assertGreaterEqual(stats['p95_upper'],nearest_rank)
        self.assertLessEqual(stats['p95_precision'],.001)
        self.assertLessEqual(oq['min_lower'],.01)
        self.assertGreaterEqual(oq['min_upper'],.01)

    def test_orthogonal_criterion_missing_only_blocks_oq(self):
        m=Model([.1,.2,.3,.96])
        stats,oq,diag=run(m,orth_available=False)
        self.assertEqual(stats['above_0_95'],1)
        self.assertIsNone(oq)
        self.assertIn('orthogonal_reason',diag)
        self.assertEqual(m.groups,['USER_EXISTING_GROUP'])

    def test_native_generation_error_leaves_no_partial_metrics(self):
        m=Model([.1,.9,.96]);m.raise_on_generate=True
        stats,oq,diag=run(m)
        self.assertIsNone(stats);self.assertIsNone(oq)
        self.assertEqual(diag['status'],'failed')
        self.assertEqual(m.groups,['USER_EXISTING_GROUP'])

    def test_cleanup_failure_never_claims_measurement_success(self):
        m=Model([.1,.9,.96]);m.raise_on_delete=True
        stats,oq,diag=run(m)
        self.assertIsNone(stats);self.assertIsNone(oq)
        self.assertEqual(diag['status'],'cleanup_failed')
        self.assertEqual(diag['temporary_selection_cleanup'],'failed')

if __name__=='__main__':unittest.main()
