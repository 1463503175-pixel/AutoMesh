# -*- coding: utf-8 -*-
"""Geometric grouping checks; no claims about flow connectivity or wafer identity."""
import unittest
from assets.geometry_rules import classify

class GroupPartitionTests(unittest.TestCase):
    def test_same_diameter_separated_locations_get_distinct_groups(self):
        faces=[]
        for i in range(96):
            # Two distant clusters along X; both have same cylindrical diameter.
            x=float(i if i<48 else i+250)
            faces.append({'id':1000+i,'body':1,'type':'Cylinder','area':28.0,
                         'centroid':[x,0,0], 'full_circle_radii':[1.5,1.5]})
        g,e=classify({'geometry_unit':'mm','faces':faces},12)
        cylinders=[v for v in g if v['category']=='circular_cylindrical']
        self.assertEqual(len(cylinders),2)
        self.assertEqual(sorted(len(x['ids']) for x in cylinders),[48,48])
        self.assertEqual(len(set(x['region_key'] for x in cylinders)),2)
        self.assertEqual(e['circular_features'],96)

    def test_continuous_hole_array_is_not_arbitrarily_halved(self):
        faces=[{'id':2000+i,'body':1,'type':'Cylinder','area':28,
                'centroid':[float(i*5),0,0], 'full_circle_radii':[1.5,1.5]}
               for i in range(96)]
        groups,_=classify({'geometry_unit':'mm','faces':faces},12)
        hole=[g for g in groups if g['category']=='circular_cylindrical']
        self.assertEqual(len(hole),1)
        self.assertEqual(len(hole[0]['ids']),96)

    def test_known_curves_form_geometry_groups_without_claiming_function(self):
        faces=[{'id':1,'body':1,'type':'SurfaceSphere','area':300,'centroid':[1,2,3]},
               {'id':2,'body':1,'type':'SurfaceSphere','area':300,'centroid':[2,3,4]},
               {'id':3,'body':1,'type':'UnknownBSpline','area':300,'centroid':[2,3,5]}]
        groups,e=classify({'geometry_unit':'mm','faces':faces},12)
        curves=[g for g in groups if g['category']=='curved_surface']
        self.assertEqual(len(curves),1)
        self.assertEqual(curves[0]['ids'],[1,2])
        self.assertEqual(curves[0]['confidence'],'low')
        self.assertTrue(all('wafer' not in c['reason'].lower() for c in curves))

if __name__=='__main__':unittest.main()
