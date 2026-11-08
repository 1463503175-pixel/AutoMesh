# coding: utf-8
import json
import pathlib
import tempfile
import unittest
from test_worker_mock import WorkerTests, ModelType

class WorksheetActionTests(unittest.TestCase):
    def run_worker(self,kind,failed,warning):
        worker=WorkerTests()
        model=ModelType()
        model.Mesh.Elements=1200
        model.Mesh.Nodes=1400
        model.Mesh.GetVolumeMeshQualityCountFailed=lambda metric:failed
        model.Mesh.GetVolumeMeshQualityWarningCountFailed=lambda metric:warning
        creations=[]
        def create(metric,kind_code):
            creations.append((metric,kind_code))
            class Created:
                Name='Worksheet_TestGroup';ObjectId=88
                Location=type('Location',(),{'Ids':[42]})()
            model.NamedSelections.Children.append(Created())
        model.Mesh.CreateMQVolumeElementsNamedSelection=create
        with tempfile.TemporaryDirectory() as path:
            snap_task={'job_id':'a'*32,'phase':'inspect'}
            worker.run_job(path,snap_task,model)
            snap=json.loads((pathlib.Path(path)/'snapshot.json').read_text())
            job={'job_id':'b'*32,'phase':kind,'expected_project':snap['project'],
                 'geometry_fingerprint':snap['fingerprint']}
            result=worker.run_job(path,job,model)
            return result,creations
    def test_zero_failed_is_not_an_error_and_does_not_modify_mesh(self):
        result,creations=self.run_worker('mark_failed',0,7)
        self.assertEqual(result['status'],'ok')
        self.assertEqual(result['summary']['status'],'zero_elements')
        self.assertEqual(result['summary']['worksheet_count'],0)
        self.assertEqual(creations,[])
    def test_warning_only_creates_warning_group(self):
        result,creations=self.run_worker('mark_warning',0,9)
        self.assertEqual(result['status'],'ok')
        self.assertEqual(creations,[('Skewness',0)])
        self.assertEqual(result['summary']['kind'],'warning')
        self.assertEqual(result['summary']['worksheet_count'],9)
    def test_missing_worksheet_api_returns_helpful_status(self):
        worker=WorkerTests();m=ModelType();m.Mesh.Elements=100
        with tempfile.TemporaryDirectory() as d:
            worker.run_job(d,{'job_id':'a'*32,'phase':'inspect'},m)
            snapshot=json.loads((pathlib.Path(d)/'snapshot.json').read_text())
            r=worker.run_job(d,{'job_id':'b'*32,'phase':'mark_failed',
                 'expected_project':snapshot['project'],
                 'geometry_fingerprint':snapshot['fingerprint']},m)
            self.assertEqual(r['status'],'ok')
            self.assertEqual(r['summary']['status'],'worksheet_unavailable')

if __name__=='__main__':unittest.main()
