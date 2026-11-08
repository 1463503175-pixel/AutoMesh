# coding: utf-8
import unittest
from assets.geometry_rules import classify, unit_to_mm

class GeometryRulesTests(unittest.TestCase):
    def test_units_are_not_assumed_meters(self):
        self.assertEqual(unit_to_mm('StandardNMM'),1.0)
        self.assertEqual(unit_to_mm('StandardMKS'),1000.0)
        self.assertEqual(unit_to_mm('StandardBIN'),25.4)
        self.assertIsNone(unit_to_mm('something_unknown'))
        self.assertEqual(unit_to_mm('Ansys.UnitSystem.StandardNMM'),1.0)
        with self.assertRaises(ValueError):
            classify({'geometry_unit':'n/a', 'faces':[{'id':1}]}, 12)

    def test_cylinder_requires_two_closed_circles(self):
        s={'geometry_unit':'mm','faces':[
             {'id':1,'area':15,'type':'GeoSurfaceCylinder','full_circle_radii':[2.0,2.0]},
             {'id':2,'area':15,'type':'GeoSurfaceCylinder','full_circle_radii':[2.0]},
             {'id':3,'area':15,'type':'GeoSurfacePlane'}]}
        g,e=classify(s,12)
        self.assertEqual(e['circular_features'],1)
        self.assertEqual(g[0]['ids'],[1])
        self.assertFalse(any(2 in x['ids'] for x in g))

    def test_opposing_surfaces_gap(self):
        # 3mm narrow gap between large planar walls in mm CAD units.
        f1={'id':10,'type':'Plane','area':10000,'centroid':[0,0,0],
            'bbox':[-50,-50,0,50,50,0],'normal':[0,0,1]}
        f2={'id':11,'type':'Plane','area':10000,'centroid':[0,0,3],
            'bbox':[-50,-50,3,50,50,3],'normal':[0,0,-1]}
        g,e=classify({'geometry_unit':'mm','faces':[f1,f2]},12)
        self.assertEqual(e['narrow_parallel_pairs'],1)
        self.assertEqual(e['gap_mm_min'],3)
        self.assertTrue(any(x['category']=='narrow_parallel' for x in g))

    def test_geometry_api_meters_is_converted_to_mm_for_sizing(self):
        a={'id':1,'type':'Plane','area':0.01,'centroid':[0,0,0],
           'bbox':[-.05,-.05,0,.05,.05,0],'normal':[0,0,1]}
        b=dict(a,id=2,centroid=[0,0,.003],bbox=[-.05,-.05,.003,.05,.05,.003],normal=[0,0,-1])
        groups,ev=classify({'geometry_unit':'m','faces':[a,b]},8)
        self.assertEqual(ev['gap_mm_min'],3)
        self.assertEqual(ev['unit_to_mm'],1000)
        self.assertTrue(any(x['category']=='narrow_parallel' and x['mm']<2 for x in groups))

    def test_same_direction_faces_not_falsely_gap(self):
        a={'id':1,'type':'Plane','area':10000,'centroid':[0,0,0],
           'bbox':[-50,-50,0,50,50,0],'normal':[0,0,1]}
        b=dict(a,id=2,centroid=[0,0,3],bbox=[-50,-50,3,50,50,3])
        g,e=classify({'geometry_unit':'mm','faces':[a,b]},12)
        self.assertEqual(e['narrow_parallel_pairs'],0)

    def test_budget_protects_many_faces(self):
        faces=[]
        for i in range(320):
            z=i*10
            for j,n in ((0,1),(1,-1)):
                loc=z+j*3
                faces.append({'id':2*i+j,'type':'Plane','area':8000,
                              'centroid':[0,0,loc],'bbox':[-50,-50,loc,50,50,loc],
                              'normal':[0,0,n]})
        groups,e=classify({'geometry_unit':'mm','faces':faces},12)
        self.assertGreater(e['narrow_parallel_pairs'],0)
        # Policy changed: DO NOT coarsen a 3mm gap below resolution requirements.
        self.assertTrue(any('32GB risk' in w for w in e['warnings']))
        self.assertFalse(any(x['category']=='narrow_parallel' for x in groups))
        self.assertGreater(e['estimated_surface_elements_proxy'],1500000)

    def test_ald_eight_boats_periodic_channels(self):
        # Eight separated stacks, 320 repeated 3mm channels in each.
        faces=[]; idx=1
        for boat in range(8):
            y=boat*130.0
            for i in range(320):
                z=i*6.0
                for j,normal in ((0,[0,0,1]),(1,[0,0,-1])):
                    zz=z+j*3
                    faces.append({'id':idx,'body':1,'type':'Plane','area':5000,
                                  'centroid':[0,y,zz],
                                  'bbox':[-40,y-40,zz,40,y+40,zz],
                                  'normal':normal})
                    idx+=1
        _,ev=classify({'geometry_unit':'mm','faces':faces},12)
        self.assertGreaterEqual(ev['narrow_parallel_pairs'],2500)
        self.assertFalse(ev['scan_truncated'])
        self.assertGreaterEqual(ev['repeated_passage_pairs'],2000)
        self.assertTrue(ev['repeated_passage_groups']>=1)

    def test_candidates_are_not_proof_of_flow(self):
        a={'id':3,'body':1,'type':'Plane','area':1600,'centroid':[0,0,0],
           'bbox':[-20,-20,0,20,20,0],'normal':[0,0,1]}
        b=dict(a,id=4,centroid=[0,0,2],bbox=[-20,-20,2,20,20,2],normal=[0,0,-1])
        _,ev=classify({'geometry_unit':'mm','faces':[a,b]},10)
        self.assertEqual(ev['narrow_parallel_pairs'],1)
        self.assertIn('candidate',ev['gap_detection'])

if __name__=='__main__': unittest.main()

class NamedRegionTests(unittest.TestCase):
    def test_named_inlet_outlet_without_hallucination(self):
        s={'geometry_unit':'mm', 'faces':[{'id':21,'type':'Plane','area':50}, {'id':22,'type':'Plane','area':50}],
           'named':[{'name':'入口_A','ids':[21]},{'name':'outlet_1','ids':[22]},{'name':'arbitrary','ids':[22]}]}
        groups,ev=classify(s,12)
        self.assertEqual(ev['named_region_groups'],2)
        self.assertEqual(groups[0]['category'],'named_inlet')
        self.assertEqual(groups[1]['category'],'named_outlet')
        self.assertEqual(ev['feature_face_fraction'],1.0)

class BoundaryTileTests(unittest.TestCase):
    def test_pair_spanning_two_tiles_is_not_lost(self):
        # base=12 -> tile=48mm; the centroid pair straddles x=24mm tile boundary.
        a={'id':1001,'body':1,'type':'Plane','area':6400.0,
           'centroid':[23.9,0,0], 'normal':[0,0,1],
           'bbox':[-16.1,-40,0,63.9,40,0]}
        b={'id':1002,'body':1,'type':'Plane','area':6400.0,
           'centroid':[24.1,0,3], 'normal':[0,0,-1],
           'bbox':[-15.9,-40,3,64.1,40,3]}
        groups,ev=classify({'geometry_unit':'mm','faces':[a,b]},12)
        self.assertEqual(ev['narrow_parallel_pairs'],1)
        self.assertEqual(ev['gap_mm_min'],3)
        self.assertTrue(any(x['category']=='narrow_parallel' for x in groups))
    def test_dense_overlaps_report_truncation(self):
        faces=[]
        for i in range(150):
            faces.append({'id':i+1,'body':1,'type':'Plane','area':10000.0,
                'centroid':[0,0,float(i%2)*3],
                'normal':[0,0,1 if i%2==0 else -1],
                'bbox':[-50,-50,float(i%2)*3,50,50,float(i%2)*3]})
        _,ev=classify({'geometry_unit':'mm','faces':faces},12)
        self.assertTrue(ev['scan_truncated'])
        self.assertFalse(ev['native_proximity_candidate_count'])

class RealisticCountSyntheticTests(unittest.TestCase):
    def test_eight_stacks_total_320_wafers_synthetic(self):
        # 8 boats x 40 wafer-like gaps; this is a synthetic topology regression,
        # NOT a substitute for the user's actual geometric dimensions.
        faces=[];fid=1
        for boat in range(8):
            y=boat*115.0
            for slot in range(40):
                z=slot*7.0
                for thickness,normvec in ((0,[0,0,1]),(3,[0,0,-1])):
                    zz=z+thickness
                    faces.append({'id':fid,'body':1,'type':'Plane','area':5200,
                        'centroid':[0,y,zz], 'normal':normvec,
                        'bbox':[-42,y-31,zz,42,y+31,zz]})
                    fid+=1
        groups,ev=classify({'geometry_unit':'mm','faces':faces},12)
        self.assertFalse(ev['scan_truncated'])
        self.assertGreaterEqual(ev['narrow_parallel_pairs'],300)
        self.assertGreater(ev['repeated_passage_pairs'],250)
        self.assertIn('gap_candidate_comparisons',ev)
