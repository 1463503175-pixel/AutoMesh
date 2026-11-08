"""Real geometry identity remains mandatory; manual controls need not force a
full expensive rescan just to READ quality. Mock-only; never calls ANSYS."""
import json
import tempfile
from pathlib import Path
import unittest
from test_editable_groups import EditableModel,Fixture,MockNamed,MockSizing

class FastQualitySafetyTests(unittest.TestCase):
    def test_manual_sizing_edit_does_not_block_readonly_quality(self):
        with tempfile.TemporaryDirectory() as td:
            model=EditableModel();model.Mesh.Elements=1500
            fixture=Fixture(model)
            self.assertEqual(fixture.run(td,'inspect',101)['status'],'ok')
            state=json.loads((Path(td)/'snapshot.json').read_text())
            c=MockSizing();c.Name='User_Manual_Sizing';c.ElementSize='2 [mm]'
            c.Location=MockNamed('Manual',[31]);model.Mesh.Children.append(c)
            result=fixture.run(td,'quality_check',102,state)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['mesh_elements'],1500)
            self.assertEqual(model.Mesh.count,0)
            self.assertEqual(c.ElementSize,'2 [mm]')

    def test_real_geometry_change_still_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            model=EditableModel();fixture=Fixture(model)
            self.assertEqual(fixture.run(td,'inspect',111)['status'],'ok')
            state=json.loads((Path(td)/'snapshot.json').read_text())
            # A change to real face area changes the fingerprint.
            one=fixture.API.DataModel.GeoData.Assemblies[0].Parts[0].Bodies[0].Faces[0]
            one.Area=300
            result=fixture.run(td,'quality_check',112,state)
            self.assertEqual(result['status'],'error')
            self.assertIn('fingerprint',result['error'])
            self.assertEqual(model.Mesh.count,0)

if __name__=='__main__':unittest.main()
