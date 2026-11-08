# -*- coding: utf-8 -*-
"""Regression: R4 refresh failed with NoneType.Children.
No real ANSYS calls; validates fail-closed and active-project Model fallback.
"""
import json
import tempfile
import pathlib
import unittest
from test_worker_mock import WORKER, WorkerTests, ModelType, API


class ReadinessRegression(unittest.TestCase):
    def _run(self, folder, model, api=None):
        p = pathlib.Path(folder)
        (p / 'job.json').write_text(json.dumps({'job_id': 'f' * 32, 'phase': 'inspect'}), encoding='utf8')
        (p / 'geometry_rules.py').write_bytes((WORKER.parent / 'geometry_rules.py').read_bytes())
        (p / 'warning_refinement.py').write_bytes((WORKER.parent / 'warning_refinement.py').read_bytes())
        ns = {
            'JOB_FILE': str(p / 'job.json'),
            'RESULT_FILE': str(p / 'last_result.json'),
            'SNAPSHOT_FILE': str(p / 'snapshot.json'),
            'ExtAPI': api or API(),
            'Model': model,
            'MeshMetricType': type('Metrics', (), {'Skewness': 'Skewness'}),
            'Quantity': lambda x: x,
            'SelectionTypeEnum': type('Selection', (), {'GeometryEntities': 1})(),
            'unicode': str,
        }
        ns['execfile'] = lambda f: exec(compile(pathlib.Path(f).read_text('utf8'), str(f), 'exec'), ns)
        ns['execfile'](WORKER)
        return json.loads((p/'last_result.json').read_text('utf8'))

    def test_valid_existing_model_still_reads_named(self):
        with tempfile.TemporaryDirectory() as t:
            r=self._run(t, ModelType())
            self.assertEqual(r['status'],'ok',r.get('error'))
            snap=json.loads((pathlib.Path(t)/'snapshot.json').read_text('utf8'))
            self.assertEqual({x['name'] for x in snap['named']},{'inlet','outlet'})

    def test_missing_mesh_fails_clearly_without_overwriting_snapshot(self):
        with tempfile.TemporaryDirectory() as t:
            old=pathlib.Path(t)/'snapshot.json'
            old.write_text('{"previous":true}', encoding='utf8')
            broken=ModelType();broken.Mesh=None
            r=self._run(t,broken)
            self.assertEqual(r['status'],'error')
            self.assertIn('模型树尚未就绪',r['error'])
            self.assertNotIn("'NoneType' object has no attribute 'Children'",r['error'])
            self.assertEqual(json.loads(old.read_text(encoding='utf8'))['previous'],True)

    def test_missing_children_properties_are_not_treated_as_empty(self):
        for missing in ('mesh_children', 'named_children'):
            with self.subTest(missing=missing), tempfile.TemporaryDirectory() as t:
                model=ModelType()
                if missing=='mesh_children':
                    model.Mesh.Children=None
                else:
                    model.NamedSelections.Children=None
                r=self._run(t,model)
                self.assertEqual(r['status'],'error')
                self.assertIn('模型树尚未就绪',r['error'])
                self.assertFalse((pathlib.Path(t)/'snapshot.json').exists())

    def test_missing_named_tree_fails_not_silent_empty_list(self):
        with tempfile.TemporaryDirectory() as t:
            broken=ModelType();broken.NamedSelections=None
            r=self._run(t,broken)
            self.assertEqual(r['status'],'error')
            self.assertIn('NamedSelections=False',r['error'])
            self.assertFalse((pathlib.Path(t)/'snapshot.json').exists())

    def test_use_active_project_model_if_global_model_not_ready(self):
        with tempfile.TemporaryDirectory() as t:
            broken=ModelType();broken.Mesh=None
            good=ModelType()
            api=API()
            api.DataModel=type('DM',(),{'Project':type('P',(),{'ProjectDirectory':r'E:\ALD\model_files','Model':good})(),
                                        'GeoData':API.DataModel.GeoData})()
            r=self._run(t, broken, api)
            self.assertEqual(r['status'],'ok',r.get('error'))
            snap=json.loads((pathlib.Path(t)/'snapshot.json').read_text('utf8'))
            self.assertEqual(len(snap['named']),2)

    def test_missing_both_roots_must_not_modify_mesh_or_names(self):
        with tempfile.TemporaryDirectory() as t:
            m=ModelType();m.Mesh=None;m.NamedSelections=None
            r=self._run(t,m)
            self.assertEqual(r['status'],'error')
            self.assertIn('模型树尚未就绪',r['error'])

if __name__=='__main__':
    unittest.main()
