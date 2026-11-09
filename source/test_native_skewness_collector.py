# coding: utf-8
"""Opt-in ACT native Skewness adapter: fixtures ONLY, no real ANSYS session."""
import tempfile
import unittest
import test_worker_mock as mocks

class NativeResult:
    Components = ['SKEWNESS']
    def __init__(self, values): self.values=values
    def SelectComponents(self, names):
        if names!=['SKEWNESS']: raise AssertionError('non-native component selected')
    def GetElementValues(self, ids): return [self.values[k] for k in ids]

class NativeAnalysis:
    def __init__(self, values): self.res=NativeResult(values)
    def GetResultsData(self): return self
    def GetResult(self, key):
        if key != 'MESH_': raise AssertionError('non-native metric result')
        return self.res

class NativeMetricTests(unittest.TestCase):
    def run_fixture(self, values, with_source=True, opt_in=True, source_values=None):
        model=mocks.ModelType()
        model.Mesh.Elements=len(values)
        model.Mesh.Nodes=len(values)*2
        model.Mesh.Minimum=mocks.V(min(values.values()))
        model.Mesh.Maximum=mocks.V(max(values.values()))
        model.Mesh.Average=mocks.V(sum(values.values())/len(values))
        project_model=type('ModelWithAnalyses',(),{'Analyses':[NativeAnalysis(source_values if source_values is not None else values)] if with_source else []})()
        data_mesh=type('MeshedElementIDs',(),{
            'ElementIds':list(values.keys()),
            'MeshRegionById':lambda self,body_id:type('Region',(),{'ElementIds':list(values.keys())})()})()
        prior_model=getattr(mocks.Data.Project,'Model',None)
        prior_get=getattr(mocks.Data,'MeshDataByName',None)
        mocks.Data.Project.Model=project_model
        mocks.Data.MeshDataByName=staticmethod(lambda name:data_mesh)
        try:
            with tempfile.TemporaryDirectory() as folder:
                result=mocks.WorkerTests().run_job(folder,{'job_id':'1'*32,'phase':'quality_check',
                    'deep_native_skewness':opt_in},model)
                self.assertEqual(model.Mesh.count,0,'deep read-only data capture generated a mesh')
                self.assertEqual(model.Mesh.Elements,len(values),'deep read-only data capture changed the mesh')
                return result
        finally:
            if prior_model is None: delattr(mocks.Data.Project,'Model')
            else: mocks.Data.Project.Model=prior_model
            if prior_get is None: delattr(mocks.Data,'MeshDataByName')
            else: mocks.Data.MeshDataByName=prior_get

    def test_complete_native_distribution_opt_in(self):
        values={1:.20,2:.90,3:.95,4:.97}
        result=self.run_fixture(values)
        self.assertEqual(result['status'],'ok',result.get('error'))
        q=result['summary']['quality']
        d=q['skewness_distribution']
        self.assertEqual(d['sample_count'],4)
        self.assertEqual(d['p95'],.97)
        self.assertEqual(d['above_0_90'],2)
        self.assertEqual(d['above_0_95'],1)
        self.assertEqual(sum(b['count'] for b in d['bins']),4)
        self.assertEqual(q['skewness_distribution_diagnostic']['status'],'complete')
        self.assertEqual(len(d['body_stats']),1)
        self.assertEqual(d['body_stats'][0]['body_id'],1)
        self.assertEqual(d['body_stats'][0]['above_0_95'],1)

    def test_missing_native_component_does_not_fabricate_distribution(self):
        result=self.run_fixture({1:.1,2:.3},with_source=False)
        self.assertEqual(result['status'],'ok',result.get('error'))
        quality=result['summary']['quality']
        self.assertNotIn('skewness_distribution',quality)
        self.assertEqual(quality['skewness_distribution_diagnostic']['status'],'unavailable')

    def test_native_source_disagrees_with_meshmetric_and_is_rejected(self):
        result=self.run_fixture({1:.2,2:.8}, source_values={1:.1,2:.9})
        self.assertEqual(result['status'],'ok',result.get('error'))
        quality=result['summary']['quality']
        self.assertNotIn('skewness_distribution',quality)
        self.assertIn('disagrees',quality['skewness_distribution_diagnostic']['reason'])

    def test_standard_quality_check_never_traverses_native_elements(self):
        result=self.run_fixture({1:.1,2:.3},with_source=False,opt_in=False)
        self.assertEqual(result['status'],'ok',result.get('error'))
        self.assertNotIn('skewness_distribution',result['summary']['quality'])
        self.assertNotIn('skewness_distribution_diagnostic',result['summary']['quality'])

if __name__=='__main__': unittest.main()
