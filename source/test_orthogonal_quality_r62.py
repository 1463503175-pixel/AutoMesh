# coding: utf-8
"""Orthogonal quality must be native ACT data or clearly marked unavailable."""
import json
import tempfile
import pathlib
import unittest
import test_worker_mock as mock


class OrthogonalQualityDiagnosticTests(unittest.TestCase):
    def run_quality(self, metric_type, value_min=0.12, value_max=0.9):
        original_metric = mock.QualityTypes
        try:
            mock.QualityTypes = metric_type
            with tempfile.TemporaryDirectory() as td:
                worker = mock.WorkerTests()
                model = mock.ModelType()
                model.Mesh.Elements = 1500
                model.Mesh.Nodes = 1900
                model.Mesh.MeshMetric = 'user-selected-original'
                model.Mesh.Minimum = mock.V(value_min)
                model.Mesh.Maximum = mock.V(value_max)
                model.Mesh.Average = mock.V(0.5)
                worker.run_job(td, {'job_id': 'a'*32, 'phase': 'inspect'}, model)
                snap = json.loads((pathlib.Path(td)/'snapshot.json').read_text())
                res = worker.run_job(td, {'job_id': 'b'*32,
                    'phase':'quality_check', 'expected_project':snap['project'],
                    'geometry_fingerprint':snap['fingerprint']}, model)
                self.assertEqual(model.Mesh.MeshMetric, 'user-selected-original')
                self.assertEqual(res['status'], 'ok')
                return res['summary']['quality']
        finally:
            mock.QualityTypes = original_metric

    def test_2024_act_missing_enum_does_not_fake_value(self):
        result = self.run_quality(type('M', (), {'Skewness': 'Skewness'}))
        self.assertNotIn('orthogonal_quality', result)
        self.assertEqual(result['orthogonal_quality_diagnostic']['status'], 'enum_not_exposed')

    def test_real_native_enum_populates_measured_quality(self):
        result = self.run_quality(type('M', (), {'Skewness': 'Skewness', 'OrthogonalQuality': 'OrthogonalQuality'}))
        self.assertEqual(result['orthogonal_quality']['min'], 0.12)
        self.assertEqual(result['orthogonal_quality_diagnostic']['status'], 'measured')

    def test_invalid_range_is_not_reported_as_real_quality(self):
        result = self.run_quality(type('M', (), {'Skewness':'Skewness', 'OrthogonalQuality':'OrthogonalQuality'}), value_min=-0.01)
        self.assertNotIn('orthogonal_quality', result)
        self.assertEqual(result['orthogonal_quality_diagnostic']['status'], 'metric_read_failed')
        self.assertIn('outside 0..1', result['orthogonal_quality_diagnostic']['reason'])

if __name__ == '__main__':
    unittest.main()
