"""SOI coordinate-system origin creation without historical-only SetOriginLocation."""
import tempfile
import unittest
from test_warning_refinement_v52 import Fixture

class PropertyCoordinate:
    OriginX = None
    OriginY = None
    OriginZ = None
    def __init__(self,parent,n):
        self.parent=parent; self.ObjectId=3000+n; self.Name=''
    def Delete(self): self.parent.Children.remove(self)

class PropertyRoot:
    def __init__(self):self.Children=[]
    def AddCoordinateSystem(self):
        obj=PropertyCoordinate(self,len(self.Children)+1)
        self.Children.append(obj)
        return obj

class TestOriginPropertyPath(unittest.TestCase):
    def test_uses_origin_xyz_instead_of_undefined_method(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.model.CoordinateSystems=PropertyRoot()
            snap=f.marked(d)
            preview=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(preview['status'],'ok',preview.get('error'))
            published=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(published['status'],'ok',published.get('error'))
            self.assertEqual(published['summary']['status'],'published')
            obj=f.model.CoordinateSystems.Children[0]
            self.assertIsNotNone(obj.OriginX)
            self.assertIsNotNone(obj.OriginY)
            self.assertIsNotNone(obj.OriginZ)

if __name__=='__main__':unittest.main()
