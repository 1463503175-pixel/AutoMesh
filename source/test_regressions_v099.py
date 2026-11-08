# coding: utf-8
"""v0.9.9 correctness fixes. ACT tests are mocked, never an ANSYS acceptance."""
import json
import pathlib
import tempfile
import unittest
from assets.geometry_rules import classify
from test_editable_groups import EditableModel, Fixture, MockNamed, MockSizing, opt_in_reviewed_area_groups


def plane(fid, body, z, normal, x):
    return {'id':fid, 'body':body, 'area':1600.0, 'type':'Plane',
            'centroid':[x,0,z],'normal':[0,0,normal],
            'bbox':[x-20,-20,z,x+20,20,z]}


class WidthSpecificGapTests(unittest.TestCase):
    def test_extreme_gap_does_not_disable_normal_gap_group(self):
        faces=[plane(1,1,0,1,0),plane(2,1,0.5,-1,0),
               plane(3,2,0,1,500),plane(4,2,3,-1,500)]
        g,e=classify({'geometry_unit':'mm','faces':faces},12)
        gaps=[x for x in g if x['category']=='narrow_parallel']
        self.assertEqual(len(gaps),1,(g,e['gap_withheld_groups']))
        self.assertEqual(gaps[0]['ids'],[3,4])
        self.assertEqual(gaps[0]['gap_mm_min'],3)
        self.assertTrue(any(z['reason']=='below_minimum_safe_sizing' for z in e['gap_withheld_groups']))
    def test_wider_cross_pair_cannot_reclaim_withheld_tight_group(self):
        faces=[]
        for i in range(320):
            a=plane(i*2+1,1,float(i*10),1,0)
            b=plane(i*2+2,1,float(i*10+3),-1,0)
            a['area']=8000.0;b['area']=8000.0
            faces.extend([a,b])
        g,e=classify({'geometry_unit':'mm','faces':faces},12)
        self.assertEqual([x for x in g if x['category']=='narrow_parallel'],[])
        self.assertTrue(any(z['reason']=='32GB_surface_proxy_exceeded' for z in e['gap_withheld_groups']))
    def test_distinct_orientation_same_body_yield_unique_names(self):
        faces=[plane(1,1,0,1,0),plane(2,1,3,-1,0)]
        # Alternate surfaces along X with normals opposite X.
        faces += [dict(plane(3,1,0,1,100),normal=[1,0,0],centroid=[100,0,0],bbox=[100,-20,-20,100,20,20]),
                  dict(plane(4,1,0,-1,100),normal=[-1,0,0],centroid=[103,0,0],bbox=[103,-20,-20,103,20,20])]
        g,e=classify({'geometry_unit':'mm','faces':faces},12)
        k=[x['region_key'] for x in g if x['category']=='narrow_parallel']
        self.assertEqual(len(k),len(set(k)))


class EffectiveControlTests(unittest.TestCase):
    def snap(self,fixture,folder):
        res=fixture.run(folder,'inspect',1)
        self.assertEqual(res['status'],'ok',res.get('error'))
        return json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))

    def test_actual_global_size_blocks_prepare_before_any_write(self):
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();m.Mesh.ElementSize='0.05 [mm]'
            f=Fixture(m);snap=self.snap(f,path)
            res=f.run(path,'prepare',2,snap,{'disabled_regions':[]})
            self.assertEqual(res['status'],'error')
            self.assertIn('Actual Meshing global ElementSize outside',res['error'])
            self.assertEqual(m.Mesh.Children,[])
            self.assertFalse(any(n.Name.startswith('AM_') for n in m.NamedSelections.Children))

    def test_automatic_global_size_allows_local_publish_but_never_claims_global_guard(self):
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();m.Mesh.ElementSize='Default'
            f=Fixture(m);snap=self.snap(f,path)
            res=f.run(path,'prepare',2,snap,{'disabled_regions':[]})
            self.assertEqual(res['status'],'ok',res.get('error'))
            budget=res['summary']['resource_preflight']
            self.assertEqual(budget['global_size_status'],'automatic_or_unresolved')
            self.assertNotIn('global_volume_proxy',budget)
            self.assertIn('NOT verified',budget['warning'])
            self.assertEqual(m.Mesh.ElementSize,'Default')
            self.assertEqual(m.Mesh.count,0)

    def test_explicit_approved_global_override_is_used_instead(self):
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();m.Mesh.ElementSize='0.05 [mm]'
            f=Fixture(m);snap=self.snap(f,path)
            res=f.run(path,'prepare',2,snap,{'disabled_regions':[]},apply_global=True)
            self.assertEqual(res['status'],'ok',res.get('error'))
            self.assertTrue(res['summary']['resource_preflight']['global_guard_applied'])
            self.assertEqual(m.Mesh.ElementSize,'12 [mm]')

    def test_manual_named_selection_sizing_is_included_in_budget(self):
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();f=Fixture(m)
            ns=MockNamed('MyManualGap',[31,32]);m.NamedSelections.Children.append(ns)
            ctrl=MockSizing();ctrl.Name='MyFineControl';ctrl.NamedSelection=ns;ctrl.ElementSize='0.05 [mm]'
            m.Mesh.Children.append(ctrl)
            snap=self.snap(f,path)
            res=f.run(path,'prepare',2,snap,{'disabled_regions':[]})
            self.assertEqual(res['status'],'error')
            self.assertIn('Invalid requested local size',res['error'])
            self.assertEqual(len(m.Mesh.Children),1)
            self.assertFalse(any(n.Name.startswith('AM_') for n in m.NamedSelections.Children))

    def test_one_region_override_does_not_change_other_region(self):
        # Area-ranked workflow replaces geometric-gap hardcoding. An override
        # to one area band must not silently overwrite a separate band.
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();f=Fixture(m)
            f.API.DataModel.GeoData.Assemblies[0].Parts[0].Bodies[0].Faces[-1].Area=30
            props=f.run(path,'inspect',10)
            self.assertEqual(props['status'],'ok',props.get('error'))
            groups=props['summary']['proposed_sizing']
            self.assertGreaterEqual(len(groups),2)
            key=next(p['group_key'] for p in groups if p['category']=='area_small')
            snap=json.loads((pathlib.Path(path)/'snapshot.json').read_text('utf8'))
            res=f.run(path,'prepare',2,snap,{'group_overrides':{key:1.0},
                        'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(res['status'],'ok',res.get('error'))
            sizes=[c.ElementSize for c in m.Mesh.Children if c.Name.startswith('AutoMesh_')]
            self.assertIn('1 [mm]',sizes)
            self.assertTrue(any(sz!='1 [mm]' for sz in sizes))

    def test_stale_group_key_blocks_before_any_write(self):
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();f=Fixture(m);snap=self.snap(f,path)
            res=f.run(path,'prepare',2,snap,{'group_overrides':{'narrow_parallel:fake':1.0}})
            self.assertEqual(res['status'],'error')
            self.assertIn('stale local group keys',res['error'])
            self.assertEqual(m.Mesh.Children,[])

if __name__ == '__main__': unittest.main()

class OverlapBudgetTests(unittest.TestCase):
    def test_finer_manual_sizing_not_lost_when_second(self):
        from assets.geometry_rules import evaluate_refinement_budget
        state={'geometry_unit':'mm','faces':[{'id':1,'area':300000}], 'bodies':[]}
        groups=[{'category':'named_inlet','ids':[1],'mm':10},
                {'category':'additional_manual','ids':[1],'mm':0.35}]
        with self.assertRaisesRegex(ValueError,'32GB surface refinement budget'):
            evaluate_refinement_budget(state,groups)
    def test_two_controls_same_size_not_double_charged(self):
        from assets.geometry_rules import evaluate_refinement_budget
        state={'geometry_unit':'mm','faces':[{'id':1,'area':10000}], 'bodies':[]}
        group={'category':'named_inlet','ids':[1],'mm':1}
        r=evaluate_refinement_budget(state,[group,dict(group,category='additional_manual')])
        self.assertEqual(r['surface_proxy'],10000)
        self.assertEqual(r['unique_scoped_faces'],1)

class GroupReviewTests(unittest.TestCase):
    def test_disable_only_one_group_keeps_other_controls(self):
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();f=Fixture(m)
            f.API.DataModel.GeoData.Assemblies[0].Parts[0].Bodies[0].Faces[-1].Area=30
            scan=f.run(path,'inspect',1)
            self.assertEqual(scan['status'],'ok',scan.get('error'))
            snap=json.loads((pathlib.Path(path)/'snapshot.json').read_text('utf8'))
            group=next(p['group_key'] for p in scan['summary']['proposed_sizing']
                       if p['category']=='area_small')
            result=f.run(path,'prepare',2,snap,{'disabled_groups':[group],
                       'enabled_groups':[key for key in opt_in_reviewed_area_groups(snap) if key!=group]})
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertFalse(any(c.Name.startswith('AutoMesh_Area') for c in m.Mesh.Children))
            self.assertTrue(any(c.Name.startswith(('AutoMesh_Review','AutoMesh_待审核')) for c in m.Mesh.Children))
            self.assertEqual(m.Mesh.count,0)

    def test_manual_component_sizing_with_valid_size_is_budgeted(self):
        with tempfile.TemporaryDirectory() as path:
            m=EditableModel();f=Fixture(m)
            ns=MockNamed('UserControlledGap',[31,32])
            m.NamedSelections.Children.append(ns)
            manual=MockSizing();manual.Name='ManualUserSizing'
            manual.NamedSelection=ns;manual.ElementSize='0.35 [mm]'
            m.Mesh.Children.append(manual)
            scan=f.run(path,'inspect',1)
            self.assertEqual(scan['status'],'ok')
            snap=json.loads((pathlib.Path(path)/'snapshot.json').read_text('utf8'))
            result=f.run(path,'prepare',2,snap,{})
            self.assertEqual(result['status'],'ok',result.get('error'))
            budget=result['summary']['resource_preflight']
            self.assertTrue(any(x['category']=='additional_manual'
                                for x in budget['sizing_groups']))
            self.assertFalse(any(c.Name.startswith('AutoMesh_Gap') for c in m.Mesh.Children))
            self.assertEqual(manual.ElementSize,'0.35 [mm]')
