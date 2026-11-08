# coding: utf-8
"""v0.10.2: full inventory and ANSYS Worksheet Face/Size compatibility.

Mock is intentionally NOT a substitute for real Workbench/Meshing integration.
"""
import pathlib
import tempfile
import unittest
import json
import re
from assets.geometry_rules import classify_face_areas
from test_editable_groups import Fixture, EditableModel, MockNamed


class AreaInventoryTests(unittest.TestCase):
    def test_more_than_28_area_bands_are_visible_not_silently_truncated(self):
        faces=[{'id':i+1,'body':1,'type':'Cylinder', 'area':1.4**i} for i in range(52)]
        grouped, evidence=classify_face_areas({'geometry_unit':'mm','faces':faces},8)
        all_ids=set(i for gr in grouped for i in gr['ids'])
        self.assertEqual(all_ids,set(range(1,53)))
        self.assertEqual(evidence['candidate_groups_omitted'],0)
        self.assertGreater(evidence['candidate_groups_published'],28)
        self.assertLessEqual(evidence['default_enabled_groups'],24)

    def test_holes_wafer_faces_and_large_panels_all_appear_in_inventory(self):
        faces=[{'id':i+1,'body':1,'type':'Cylinder','area':26.5} for i in range(400)]
        faces +=[{'id':1000+i,'body':1,'type':'Plane','area':8500.0} for i in range(320)]
        faces +=[{'id':2000+i,'body':1,'type':'Plane','area':140000.0+i*10000} for i in range(3)]
        state={'geometry_unit':'mm','faces':faces,'named':[{'name':'4inch','ids':[1000+i for i in range(320)]}]}
        groups,e=classify_face_areas(state,8)
        area=[g for g in groups if g['category']!='named_existing']
        self.assertEqual(set(i for g in area for i in g['ids']),set(f['id'] for f in faces))
        self.assertTrue(any(g['surface_family']=='Cylinder' and len(g['ids'])==400 for g in area))
        self.assertTrue(any(g['category']=='named_existing' and g['region_key']=='4inch' for g in groups))
        self.assertTrue(all(not g['default_enabled'] for g in area if g['area_mm2_min']>100000))
        self.assertEqual(e['named_faces_in_area_inventory'],320)


class MockWorksheet(object):
    def __init__(self):self.rows=[]
    def Add(self,entry):self.rows.append(type('Criterion',(),{})())
    def __getitem__(self,i):return self.rows[i]


class MockNativeNamed(MockNamed):
    def __init__(self,children,faces):
        super().__init__('(new)',[])
        self.SendToSolver=True
        self.ScopingMethod=None
        self.GenerationCriteria=MockWorksheet()
        self.children=children
        self.faces=faces
        self.generate_called=False
    def Generate(self):
        self.generate_called=True
        row=self.GenerationCriteria[0]
        def mm2(value):
            return float(re.match(r'^([+\-0-9.eE]+)',str(value)).group(1))
        lo,hi=mm2(row.LowerBound),mm2(row.UpperBound)
        self.Location.Ids=[int(f.Id) for f in self.faces if lo<=float(f.Area)<=hi]
    def Delete(self):
        self.children.remove(self)


class NativeAreaPublishingTests(unittest.TestCase):
    def test_publishes_verified_native_worksheet_area_ranges(self):
        with tempfile.TemporaryDirectory() as folder:
            model=EditableModel();fixture=Fixture(model)
            faces=fixture.API.DataModel.GeoData.Assemblies[0].Parts[0].Bodies[0].Faces
            faces[0].Area=10000
            faces[1].Area=14000
            faces[2].Area=26.5
            faces[3].Area=26.5
            created=[]
            def add_native():
                obj=MockNativeNamed(model.NamedSelections.Children, faces)
                model.NamedSelections.Children.append(obj)
                created.append(obj)
                return obj
            model.AddNamedSelection=add_native
            class Enums: pass
            def en(**kwargs):
                return type('EnumValue',(),kwargs)()
            fixture.native_namespace={
              'GeometryDefineByType':en(Component='Component',Worksheet='Worksheet'),
              'SelectionType':en(GeoFace='GeoFace'),
              'SelectionCriterionType':en(Size='Size'),
              'SelectionActionType':en(Add='Add'),
              'SelectionOperatorType':en(RangeInclude='RangeInclude')}
            scan=fixture.run(folder,'inspect',1)
            self.assertEqual(scan['status'],'ok',scan.get('error'))
            snap=json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))
            groups=scan['summary']['proposed_sizing']
            target=next(g for g in groups if g['category']=='area_small' and g['face_count']==2)
            pub=fixture.run(folder,'publish_names',2,snap,{
                'enabled_groups':[target['group_key']]})
            self.assertEqual(pub['status'],'ok',pub.get('error'))
            selected=[g for g in pub['summary']['named_groups'] if g['status']=='created']
            self.assertEqual(len(selected),1)
            self.assertEqual(selected[0]['scope_method'],'native_worksheet_face_area')
            self.assertEqual(set(created[0].Location.Ids),{31,32})
            self.assertTrue(created[0].generate_called)
            self.assertFalse(created[0].SendToSolver)
            self.assertEqual(model.Mesh.count,0)

    def test_ambiguous_area_uses_exact_face_scoping_not_extra_faces(self):
        # Two bodies share the same face area; native global worksheet would
        # inadvertently select both. In that case explicit verified IDs win.
        from test_editable_groups import opt_in_reviewed_area_groups
        with tempfile.TemporaryDirectory() as folder:
            model=EditableModel();fixture=Fixture(model)
            snapresult=fixture.run(folder,'inspect',1)
            self.assertEqual(snapresult['status'],'ok')
            snap=json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))
            enabled=opt_in_reviewed_area_groups(snap)
            pub=fixture.run(folder,'publish_names',2,snap,{'enabled_groups':enabled})
            self.assertEqual(pub['status'],'ok',pub.get('error'))
            selected=pub['summary']['named_groups']
            self.assertTrue(all(g['scope_method'] in ('verified_face_ids','preserved') for g in selected))
            self.assertEqual(model.Mesh.count,0)

if __name__=='__main__':unittest.main()

class NamedOverlapTests(unittest.TestCase):
    def test_user_named_holes_remain_visible_in_area_inventory_and_am_names(self):
        with tempfile.TemporaryDirectory() as folder:
            model=EditableModel();fixture=Fixture(model)
            faces=fixture.API.DataModel.GeoData.Assemblies[0].Parts[0].Bodies[0].Faces
            faces[0].Area=12000.0;faces[1].Area=15000.0
            faces[2].Area=24.0;faces[3].Area=24.0
            model.NamedSelections.Children.append(MockNamed('holes_hand', [31,32]))
            scan=fixture.run(folder,'inspect',1)
            self.assertEqual(scan['status'],'ok',scan.get('error'))
            snap=json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))
            g=next(g for g in scan['summary']['proposed_sizing']
                   if g['category']=='area_small' and g['face_count']==2)
            pub=fixture.run(folder,'publish_names',2,snap,{
                'enabled_groups':[g['group_key']]})
            self.assertEqual(pub['status'],'ok',pub.get('error'))
            # Existing manual NS is preserved; independent AM_* allows review
            # of the same faces, but never sends this duplicate to Fluent.
            self.assertEqual(next(n for n in model.NamedSelections.Children
                                  if n.Name=='holes_hand').Location.Ids,[31,32])
            groups=[n for n in model.NamedSelections.Children if n.Name.startswith('AM_')]
            self.assertTrue(any(set(n.Location.Ids)=={31,32} for n in groups))
            self.assertTrue(all(n.SendToSolver is False for n in groups))
            self.assertEqual(model.Mesh.count,0)
